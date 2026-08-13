// SPDX-License-Identifier: MIT
pragma solidity ^0.8.27;

import { VeilCreditInstructionSender } from "../contracts/InstructionSender.sol";
import { ITeeExtensionRegistry } from "../contracts/interfaces/ITeeExtensionRegistry.sol";
import { ITeeMachineRegistry } from "../contracts/interfaces/ITeeMachineRegistry.sol";

interface Vm {
    function prank(address caller) external;
    function warp(uint256 timestamp) external;
}

contract MockTeeRegistries is ITeeExtensionRegistry, ITeeMachineRegistry {
    uint256 public override nextPublicExtensionId = 0x10001;
    address public instructionSender;
    address public selectedTee = address(0x1111);
    address public lastTeeId;
    bytes32 public lastCommand;
    bytes public lastMessage;
    uint256 private instructionNonce;

    function setInstructionSender(address sender) external {
        instructionSender = sender;
    }

    function setSelectedTee(address teeId) external {
        selectedTee = teeId;
    }

    function getTeeExtensionInstructionsSender(uint256 extensionId) external view returns (address) {
        return extensionId == 0x10000 ? instructionSender : address(0);
    }

    function getRandomTeeIds(uint256, uint256 count) external view returns (address[] memory teeIds) {
        require(count == 1, "unexpected count");
        teeIds = new address[](1);
        teeIds[0] = selectedTee;
    }

    function sendInstructions(
        address[] calldata teeIds,
        TeeInstructionParams calldata params
    ) external payable returns (bytes32 instructionId) {
        require(teeIds.length == 1, "unexpected tee count");
        lastTeeId = teeIds[0];
        lastCommand = params.opCommand;
        lastMessage = params.message;
        instructionId = keccak256(abi.encode(++instructionNonce, params.opCommand, params.message));
    }
}

contract VeilCreditInstructionSenderTest {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function testBindsEnvelopesPinsTeeAndEnforcesClose() external {
        MockTeeRegistries registry = new MockTeeRegistries();
        VeilCreditInstructionSender sender = new VeilCreditInstructionSender(
            ITeeExtensionRegistry(address(registry)),
            ITeeMachineRegistry(address(registry))
        );
        registry.setInstructionSender(address(sender));
        sender.setExtensionId();
        sender.setAuctionDuration(10);

        address asset = address(0xCAFE);
        bytes memory encryptedOpen = hex"010203";
        (bytes32 requestId,) = sender.openRequest(encryptedOpen, asset, 5_000);

        require(registry.lastTeeId() == address(0x1111), "OPEN used wrong TEE");
        VeilCreditInstructionSender.OpenInstruction memory opened = abi.decode(
            registry.lastMessage(),
            (VeilCreditInstructionSender.OpenInstruction)
        );
        require(opened.requestId == requestId, "OPEN requestId not bound");
        require(opened.borrower == address(this), "OPEN borrower not bound");
        require(opened.asset == asset, "OPEN asset not bound");
        require(opened.principal == 5_000 && opened.collateral == 0, "OPEN amounts not bound");
        require(keccak256(opened.encryptedRequest) == keccak256(encryptedOpen), "OPEN ciphertext changed");

        // A later random selection must not move this request to another machine.
        registry.setSelectedTee(address(0x2222));
        bytes memory encryptedQuote = hex"040506";
        sender.sendQuote(requestId, encryptedQuote);
        require(registry.lastTeeId() == address(0x1111), "QUOTE did not use pinned TEE");
        VeilCreditInstructionSender.QuoteInstruction memory quoted = abi.decode(
            registry.lastMessage(),
            (VeilCreditInstructionSender.QuoteInstruction)
        );
        require(quoted.requestId == requestId, "QUOTE requestId not bound");
        require(quoted.lender == address(this), "QUOTE lender not bound");
        require(keccak256(quoted.encryptedQuote) == keccak256(encryptedQuote), "QUOTE ciphertext changed");

        (bool premature,) = address(sender).call(abi.encodeCall(sender.sendFinalize, (requestId)));
        require(!premature, "FINALIZE succeeded before close");

        vm.warp(block.timestamp + 10);
        vm.prank(address(0xBEEF));
        (bool unauthorized,) = address(sender).call(abi.encodeCall(sender.sendFinalize, (requestId)));
        require(!unauthorized, "non-borrower finalized");

        (bool lateQuote,) = address(sender).call(
            abi.encodeCall(sender.sendQuote, (requestId, encryptedQuote))
        );
        require(!lateQuote, "QUOTE succeeded after close");

        sender.sendFinalize(requestId);
        require(registry.lastTeeId() == address(0x1111), "FINALIZE did not use pinned TEE");
        require(registry.lastMessage().length == 32, "FINALIZE was not raw bytes32");
        require(abi.decode(registry.lastMessage(), (bytes32)) == requestId, "FINALIZE requestId changed");
    }
}
