// Package helloworld preserves source compatibility for the scaffold's optional
// integration tests. New code should import pkg/contracts/veilcredit directly.
package helloworld

import "extension-scaffold/tools/pkg/contracts/veilcredit"

type HelloWorldInstructionSender = veilcredit.VeilCreditInstructionSender

var HelloWorldInstructionSenderMetaData = veilcredit.VeilCreditInstructionSenderMetaData
var DeployHelloWorldInstructionSender = veilcredit.DeployVeilCreditInstructionSender
var NewHelloWorldInstructionSender = veilcredit.NewVeilCreditInstructionSender
