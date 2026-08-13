// SPDX-License-Identifier: MIT
pragma solidity ^0.8.27;

// TODO: Replace local interfaces with imports from flare-smart-contracts-v2 once published as a package.
import { ITeeExtensionRegistry } from "./interfaces/ITeeExtensionRegistry.sol";
import { ITeeMachineRegistry } from "./interfaces/ITeeMachineRegistry.sol";

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

/// @title VeilCreditInstructionSender
/// @notice Confidential credit requests and quotes on Flare Confidential Compute, with
///         optional FXRP escrow and TEE-authorized on-chain settlement.
/// @dev OPEN and QUOTE wrap ciphertext in an ABI envelope containing only already-public
///      metadata. The selected TEE verifies that decrypted identity and amount agree with
///      that envelope, while private underwriting data and pricing remain encrypted.
///
/// DO NOT MODIFY: constructor registry arguments, setExtensionId(), _getExtensionId()
contract VeilCreditInstructionSender {
    // forge-lint: disable-next-line(unsafe-typecast)
    bytes32 public constant OP_TYPE_CREDIT = bytes32("CREDIT");
    // forge-lint: disable-next-line(unsafe-typecast)
    bytes32 public constant OP_COMMAND_OPEN = bytes32("OPEN");
    // forge-lint: disable-next-line(unsafe-typecast)
    bytes32 public constant OP_COMMAND_QUOTE = bytes32("QUOTE");
    // forge-lint: disable-next-line(unsafe-typecast)
    bytes32 public constant OP_COMMAND_FINALIZE = bytes32("FINALIZE");

    bytes32 public constant REQUEST_DOMAIN = keccak256("VEILCREDIT_REQUEST_V1");
    bytes32 public constant FINALIZATION_DOMAIN = keccak256("VEILCREDIT_FINALIZATION_V1");
    bytes32 public constant COLLATERAL_RELEASE_DOMAIN = keccak256("VEILCREDIT_COLLATERAL_RELEASE_V1");

    uint64 public constant MIN_AUCTION_DURATION = 1;
    uint64 public constant MAX_AUCTION_DURATION = 30 days;

    ITeeExtensionRegistry public immutable TEE_EXTENSION_REGISTRY;
    ITeeMachineRegistry public immutable TEE_MACHINE_REGISTRY;

    /// @notice Account allowed to configure the TEE result signer.
    address public immutable owner;

    /// @notice Address recovered from signed finalization and collateral-release payloads.
    /// @dev Experimental relay surface. This signer is not configured by the current FCC
    ///      ActionResult flow; production deployments must wire and test it explicitly.
    address public teeSigner;

    /// @notice Duration applied to newly opened auctions. Existing closesAt values never change.
    uint64 public auctionDuration = 1 hours;

    /// @notice First public extension ID. IDs below this value are reserved by FCC.
    uint256 private constant FIRST_PUBLIC_EXTENSION_ID = 0x10000;
    uint256 private _extensionId;
    uint256 private _entered = 1;

    enum RequestStatus {
        None,
        Open,
        Finalized,
        Settled,
        Cancelled
    }

    struct CreditRequest {
        address borrower;
        address asset;
        address teeId;
        uint256 principal;
        uint256 collateral;
        uint64 openedAt;
        uint64 closesAt;
        RequestStatus status;
        address winningLender;
        uint256 aprBps;
        uint256 amountFxrp;
        bytes32 commitment;
        bool funded;
    }

    /// @dev ABI envelope authenticated as part of the FCC instruction. Only encryptedRequest
    ///      is confidential; every other field is already public in contract state.
    struct OpenInstruction {
        bytes32 requestId;
        address borrower;
        address asset;
        uint256 principal;
        uint256 collateral;
        bytes encryptedRequest;
    }

    /// @dev ABI envelope binds a quote's claimed lender and request to the on-chain caller.
    struct QuoteInstruction {
        bytes32 requestId;
        address lender;
        bytes encryptedQuote;
    }

    mapping(address borrower => uint256 nonce) public nextRequestNonce;
    mapping(bytes32 requestId => CreditRequest request) public requests;
    mapping(bytes32 digest => bool used) public usedAuthorizations;

    event TeeSignerUpdated(address indexed previousSigner, address indexed newSigner);
    event AuctionDurationUpdated(uint64 previousDuration, uint64 newDuration);
    event CreditRequestOpened(
        bytes32 indexed requestId,
        bytes32 indexed instructionId,
        address indexed borrower,
        address asset,
        uint256 principal,
        uint256 collateral,
        bytes32 ciphertextHash
    );
    event CreditQuoteSubmitted(
        bytes32 indexed requestId,
        bytes32 indexed instructionId,
        address indexed submitter,
        bytes32 ciphertextHash
    );
    event CreditFinalizationRequested(
        bytes32 indexed instructionId,
        bytes32 indexed requestId,
        address indexed submitter
    );
    event CreditFinalized(
        bytes32 indexed requestId,
        address indexed borrower,
        address indexed winningLender,
        uint256 aprBps,
        uint256 amountFxrp,
        bytes32 commitment
    );
    event LoanFunded(bytes32 indexed requestId, address indexed lender, address indexed borrower, uint256 amount);
    event CollateralReleased(bytes32 indexed requestId, address indexed recipient, uint256 amount, bool defaulted);
    event CreditRequestCancelled(bytes32 indexed requestId);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    modifier nonReentrant() {
        require(_entered == 1, "reentrant call");
        _entered = 2;
        _;
        _entered = 1;
    }

    /// @notice Initializes the contract with the two FCC registry interfaces.
    /// @dev The deployment tooling passes the FlareTeeManager diamond for both arguments.
    constructor(
        ITeeExtensionRegistry _teeExtensionRegistry,
        ITeeMachineRegistry _teeMachineRegistry
    ) {
        require(address(_teeExtensionRegistry) != address(0), "TeeExtensionRegistry cannot be zero address");
        require(address(_teeMachineRegistry) != address(0), "TeeMachineRegistry cannot be zero address");
        require(address(_teeExtensionRegistry).code.length > 0, "TeeExtensionRegistry has no code");
        require(address(_teeMachineRegistry).code.length > 0, "TeeMachineRegistry has no code");
        TEE_EXTENSION_REGISTRY = _teeExtensionRegistry;
        TEE_MACHINE_REGISTRY = _teeMachineRegistry;
        owner = msg.sender;
    }

    /// @notice Finds and caches this contract's extension ID. Can only be set once.
    /// DO NOT MODIFY this function.
    function setExtensionId() external {
        require(_extensionId == 0, "Extension ID already set.");

        uint256 c = TEE_EXTENSION_REGISTRY.nextPublicExtensionId();
        for (uint256 i = FIRST_PUBLIC_EXTENSION_ID; i < c; ++i) {
            if (TEE_EXTENSION_REGISTRY.getTeeExtensionInstructionsSender(i) == address(this)) {
                _extensionId = i;
                return;
            }
        }
        revert("Extension ID not found.");
    }

    /// @notice Sets the signer used for TEE-authorized settlement exactly once.
    /// @dev This does not affect FCC result delivery; it only gates relay functions below.
    function setTeeSigner(address _teeSigner) external onlyOwner {
        require(teeSigner == address(0), "TEE signer already set");
        require(_teeSigner != address(0), "zero TEE signer");
        emit TeeSignerUpdated(teeSigner, _teeSigner);
        teeSigner = _teeSigner;
    }

    /// @notice Changes the duration used only for requests opened after this transaction.
    function setAuctionDuration(uint64 newDuration) external onlyOwner {
        require(
            newDuration >= MIN_AUCTION_DURATION && newDuration <= MAX_AUCTION_DURATION,
            "invalid auction duration"
        );
        emit AuctionDurationUpdated(auctionDuration, newDuration);
        auctionDuration = newDuration;
    }

    /// @notice Computes the ID the next request from `borrower` will receive.
    /// @dev Clients put this value in the encrypted OPEN JSON before submitting the ciphertext.
    function previewRequestId(
        address borrower,
        address asset,
        uint256 principal,
        uint256 collateral
    ) public view returns (bytes32) {
        return keccak256(
            abi.encode(
                REQUEST_DOMAIN,
                block.chainid,
                address(this),
                borrower,
                asset,
                principal,
                collateral,
                nextRequestNonce[borrower]
            )
        );
    }

    /// @notice Opens a confidential request without locking collateral.
    /// @param encryptedRequest ECIES ciphertext of the OPEN JSON payload for the selected TEE.
    /// @param asset Requested loan asset (FXRP in the production flow).
    /// @param principal Requested amount in the asset's smallest unit.
    function openRequest(
        bytes calldata encryptedRequest,
        address asset,
        uint256 principal
    ) external payable nonReentrant returns (bytes32 requestId, bytes32 instructionId) {
        return _openRequest(encryptedRequest, asset, principal, 0);
    }

    /// @notice Opens a confidential request and escrows ERC-20 collateral.
    /// @param encryptedRequest ECIES ciphertext of the OPEN JSON payload for the selected TEE.
    /// @param asset FXRP-compatible ERC-20 used for both the requested loan and escrow.
    /// @param principal Requested amount in the asset's smallest unit.
    /// @param collateral Amount pulled from the borrower into this contract.
    function openRequestWithCollateral(
        bytes calldata encryptedRequest,
        address asset,
        uint256 principal,
        uint256 collateral
    ) external payable nonReentrant returns (bytes32 requestId, bytes32 instructionId) {
        require(collateral > 0, "zero collateral");
        return _openRequest(encryptedRequest, asset, principal, collateral);
    }

    /// @notice Sends a lender's encrypted quote to the request's pinned TEE.
    /// @param requestId Public request identifier returned by openRequest.
    /// @param encryptedQuote ECIES ciphertext of the QUOTE JSON payload for the selected TEE.
    function sendQuote(
        bytes32 requestId,
        bytes calldata encryptedQuote
    ) external payable returns (bytes32 instructionId) {
        CreditRequest storage request = requests[requestId];
        require(request.status == RequestStatus.Open, "request not open");
        // Auction boundaries intentionally use the chain timestamp; small validator drift
        // cannot alter quote ordering and is negligible relative to configured durations.
        // forge-lint: disable-next-line(block-timestamp)
        require(block.timestamp < request.closesAt, "auction closed");
        require(encryptedQuote.length > 0, "empty quote");
        bytes memory message = abi.encode(QuoteInstruction({
            requestId: requestId,
            lender: msg.sender,
            encryptedQuote: encryptedQuote
        }));
        instructionId = _sendInstructionTo(request.teeId, OP_COMMAND_QUOTE, message);
        emit CreditQuoteSubmitted(requestId, instructionId, msg.sender, keccak256(encryptedQuote));
    }

    /// @notice Requests confidential winner selection for a request.
    /// @dev FINALIZE is the raw ABI bytes32 requestId. Only the borrower may close the
    ///      auction, and only after the immutable per-request closesAt timestamp.
    function sendFinalize(bytes32 requestId) external payable returns (bytes32 instructionId) {
        CreditRequest storage request = requests[requestId];
        require(request.status == RequestStatus.Open, "request not open");
        require(msg.sender == request.borrower, "not borrower");
        // forge-lint: disable-next-line(block-timestamp)
        require(block.timestamp >= request.closesAt, "auction still open");
        instructionId = _sendInstructionTo(request.teeId, OP_COMMAND_FINALIZE, abi.encode(requestId));
        emit CreditFinalizationRequested(instructionId, requestId, msg.sender);
    }

    /// @notice Records a TEE-authorized winning quote on-chain.
    /// @dev EXPERIMENTAL: the signer signs the EIP-191 personal-sign digest of
    ///      `finalizationDigest(...)`. The current extension does not yet produce this custom
    ///      signature. Anyone may relay a valid future signature.
    function relayFinalization(
        bytes32 requestId,
        address winningLender,
        uint256 aprBps,
        uint256 amountFxrp,
        bytes32 commitment,
        bytes calldata signature
    ) external {
        CreditRequest storage request = requests[requestId];
        require(request.status == RequestStatus.Open, "request not open");
        // forge-lint: disable-next-line(block-timestamp)
        require(block.timestamp >= request.closesAt, "auction still open");
        require(teeSigner != address(0), "TEE signer not configured");
        require(winningLender != address(0), "zero lender");
        require(amountFxrp > 0 && amountFxrp <= request.principal, "invalid amount");
        require(aprBps > 0 && aprBps <= 1_000_000, "invalid APR");
        require(commitment != bytes32(0), "zero commitment");

        bytes32 digest = finalizationDigest(requestId, winningLender, aprBps, amountFxrp, commitment);
        require(!usedAuthorizations[digest], "authorization already used");
        require(_recoverSigner(digest, signature) == teeSigner, "invalid TEE signature");

        usedAuthorizations[digest] = true;
        request.status = RequestStatus.Finalized;
        request.winningLender = winningLender;
        request.aprBps = aprBps;
        request.amountFxrp = amountFxrp;
        request.commitment = commitment;

        emit CreditFinalized(
            requestId,
            request.borrower,
            winningLender,
            aprBps,
            amountFxrp,
            commitment
        );
    }

    /// @notice Funds a finalized loan by pulling FXRP from the winning lender to the borrower.
    /// @dev The lender must approve this contract for `amountFxrp` first.
    function fundLoan(bytes32 requestId) external nonReentrant {
        CreditRequest storage request = requests[requestId];
        require(request.status == RequestStatus.Finalized, "request not finalized");
        require(msg.sender == request.winningLender, "not winning lender");
        require(!request.funded, "already funded");
        require(request.asset.code.length > 0, "asset has no code");

        request.funded = true;
        _safeTransferFrom(request.asset, msg.sender, request.borrower, request.amountFxrp);
        emit LoanFunded(requestId, msg.sender, request.borrower, request.amountFxrp);
    }

    /// @notice Releases escrow after repayment, default, or an unfunded finalization.
    /// @dev On repayment/cancellation collateral returns to the borrower. On a TEE-confirmed
    ///      default it goes to the winning lender. The authorization is domain-separated.
    function relayCollateralRelease(
        bytes32 requestId,
        bool defaulted,
        bytes calldata signature
    ) external nonReentrant {
        CreditRequest storage request = requests[requestId];
        require(request.status == RequestStatus.Finalized, "request not finalized");
        require(request.collateral > 0, "no collateral");
        require(!defaulted || request.funded, "unfunded loan cannot default");
        require(teeSigner != address(0), "TEE signer not configured");

        bytes32 digest = collateralReleaseDigest(requestId, defaulted);
        require(!usedAuthorizations[digest], "authorization already used");
        require(_recoverSigner(digest, signature) == teeSigner, "invalid TEE signature");

        usedAuthorizations[digest] = true;
        request.status = RequestStatus.Settled;
        address recipient = defaulted ? request.winningLender : request.borrower;
        uint256 amount = request.collateral;
        request.collateral = 0;
        _safeTransfer(request.asset, recipient, amount);
        emit CollateralReleased(requestId, recipient, amount, defaulted);
    }

    /// @notice Cancels an open request and returns any escrowed collateral.
    function cancelRequest(bytes32 requestId) external nonReentrant {
        CreditRequest storage request = requests[requestId];
        require(request.status == RequestStatus.Open, "request not open");
        require(msg.sender == request.borrower, "not borrower");

        request.status = RequestStatus.Cancelled;
        uint256 amount = request.collateral;
        request.collateral = 0;
        if (amount > 0) {
            _safeTransfer(request.asset, request.borrower, amount);
        }
        emit CreditRequestCancelled(requestId);
    }

    /// @notice Hash authenticated by the TEE for `relayFinalization`.
    function finalizationDigest(
        bytes32 requestId,
        address winningLender,
        uint256 aprBps,
        uint256 amountFxrp,
        bytes32 commitment
    ) public view returns (bytes32) {
        CreditRequest storage request = requests[requestId];
        return keccak256(
            abi.encode(
                FINALIZATION_DOMAIN,
                block.chainid,
                address(this),
                requestId,
                request.borrower,
                request.asset,
                request.principal,
                request.collateral,
                winningLender,
                aprBps,
                amountFxrp,
                commitment
            )
        );
    }

    /// @notice Hash authenticated by the TEE for `relayCollateralRelease`.
    function collateralReleaseDigest(bytes32 requestId, bool defaulted) public view returns (bytes32) {
        CreditRequest storage request = requests[requestId];
        return keccak256(
            abi.encode(
                COLLATERAL_RELEASE_DOMAIN,
                block.chainid,
                address(this),
                requestId,
                request.borrower,
                request.winningLender,
                request.asset,
                request.collateral,
                request.funded,
                defaulted
            )
        );
    }

    function _openRequest(
        bytes calldata encryptedRequest,
        address asset,
        uint256 principal,
        uint256 collateral
    ) internal returns (bytes32 requestId, bytes32 instructionId) {
        require(encryptedRequest.length > 0, "empty request");
        require(asset != address(0), "zero asset");
        require(principal > 0, "zero principal");

        requestId = previewRequestId(msg.sender, asset, principal, collateral);
        require(requests[requestId].status == RequestStatus.None, "request exists");
        address[] memory teeIds = TEE_MACHINE_REGISTRY.getRandomTeeIds(_getExtensionId(), 1);
        require(teeIds.length == 1 && teeIds[0] != address(0), "TEE unavailable");
        uint256 closeTimestamp = block.timestamp + auctionDuration;
        require(closeTimestamp <= type(uint64).max, "close timestamp overflow");
        unchecked {
            nextRequestNonce[msg.sender]++;
        }

        requests[requestId] = CreditRequest({
            borrower: msg.sender,
            asset: asset,
            teeId: teeIds[0],
            principal: principal,
            collateral: collateral,
            openedAt: uint64(block.timestamp),
            // closeTimestamp was checked against type(uint64).max immediately above.
            // forge-lint: disable-next-line(unsafe-typecast)
            closesAt: uint64(closeTimestamp),
            status: RequestStatus.Open,
            winningLender: address(0),
            aprBps: 0,
            amountFxrp: 0,
            commitment: bytes32(0),
            funded: false
        });

        if (collateral > 0) {
            require(asset.code.length > 0, "asset has no code");
            _safeTransferFrom(asset, msg.sender, address(this), collateral);
        }

        // Bind decrypted identity and amount to public on-chain facts without exposing the
        // private underwriting packet carried in encryptedRequest.
        bytes memory message = abi.encode(OpenInstruction({
            requestId: requestId,
            borrower: msg.sender,
            asset: asset,
            principal: principal,
            collateral: collateral,
            encryptedRequest: encryptedRequest
        }));
        instructionId = _sendInstruction(teeIds, OP_COMMAND_OPEN, message);
        emit CreditRequestOpened(
            requestId,
            instructionId,
            msg.sender,
            asset,
            principal,
            collateral,
            keccak256(encryptedRequest)
        );
    }

    function _sendInstructionTo(
        address teeId,
        bytes32 opCommand,
        bytes memory message
    ) internal returns (bytes32) {
        require(teeId != address(0), "TEE unavailable");
        address[] memory teeIds = new address[](1);
        teeIds[0] = teeId;
        return _sendInstruction(teeIds, opCommand, message);
    }

    function _sendInstruction(
        address[] memory teeIds,
        bytes32 opCommand,
        bytes memory message
    ) internal returns (bytes32) {
        address[] memory cosigners = new address[](0);

        ITeeExtensionRegistry.TeeInstructionParams memory params = ITeeExtensionRegistry.TeeInstructionParams({
            opType: OP_TYPE_CREDIT,
            opCommand: opCommand,
            message: message,
            cosigners: cosigners,
            cosignersThreshold: 0,
            claimBackAddress: msg.sender
        });

        return TEE_EXTENSION_REGISTRY.sendInstructions{value: msg.value}(teeIds, params);
    }

    /// @notice Returns the cached extension ID, reverting if not yet set.
    /// DO NOT MODIFY this function.
    function _getExtensionId() internal view returns (uint256) {
        require(_extensionId != 0, "Extension ID is not set.");
        return _extensionId;
    }

    function _recoverSigner(bytes32 digest, bytes calldata signature) private pure returns (address) {
        require(signature.length == 65, "invalid signature length");

        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := calldataload(signature.offset)
            s := calldataload(add(signature.offset, 32))
            v := byte(0, calldataload(add(signature.offset, 64)))
        }

        // Reject malleable signatures (secp256k1n / 2) and non-standard recovery IDs.
        require(uint256(s) <= 0x7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0, "invalid signature s");
        if (v < 27) v += 27;
        require(v == 27 || v == 28, "invalid signature v");

        bytes32 ethSignedDigest = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", digest)
        );
        return ecrecover(ethSignedDigest, v, r, s);
    }

    function _safeTransferFrom(address token, address from, address to, uint256 amount) private {
        (bool success, bytes memory result) = token.call(
            abi.encodeCall(IERC20.transferFrom, (from, to, amount))
        );
        require(success && (result.length == 0 || abi.decode(result, (bool))), "transferFrom failed");
    }

    function _safeTransfer(address token, address to, uint256 amount) private {
        (bool success, bytes memory result) = token.call(abi.encodeCall(IERC20.transfer, (to, amount)));
        require(success && (result.length == 0 || abi.decode(result, (bool))), "transfer failed");
    }
}
