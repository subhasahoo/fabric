/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chaincode

import (
	"fmt"

	"github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var chaincodeInstantiateCmd *cobra.Command

const instantiateCmdName = "instantiate"

const instantiateDesc = "Deploy the specified chaincode to the network."

// instantiateCmd returns the cobra command for Chaincode Deploy
func instantiateCmd(cf *ChaincodeCmdFactory) *cobra.Command {
	chaincodeInstantiateCmd = &cobra.Command{
		Use:       instantiateCmdName,
		Short:     fmt.Sprint(instantiateDesc),
		Long:      fmt.Sprint(instantiateDesc),
		ValidArgs: []string{"1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cf1 := cf
			if cf1 == nil {
				var err error
				cf1, err = InitCmdFactory(true, true)
				if err != nil {
					return err
				}
			}
			return chaincodeDeploy(cf1, func() error {
				return lsccInstantiate(cmd, cf1)
			})
		},
	}
	flagList := []string{
		"lang",
		"ctor",
		"name",
		"channelID",
		"version",
		"policy",
		"escc",
		"vscc",
		"resourceEnvelopeSavePath",
		"resourceEnvelopeLoadPath",
	}
	attachFlags(chaincodeInstantiateCmd, flagList)
	return chaincodeInstantiateCmd
}

//instantiate the command via Endorser
func instantiate(cmd *cobra.Command, cf *ChaincodeCmdFactory) (*common.Envelope, error) {
	spec, err := getChaincodeSpec(cmd)
	if err != nil {
		return nil, err
	}

	cds, err := getChaincodeDeploymentSpec(spec, false)
	if err != nil {
		return nil, fmt.Errorf("Error getting chaincode code %s: %s", chainFuncName, err)
	}

	creator, err := cf.Signer.Serialize()
	if err != nil {
		return nil, fmt.Errorf("Error serializing identity for %s: %s", cf.Signer.GetIdentifier(), err)
	}

	prop, _, err := utils.CreateDeployProposalFromCDS(channelID, cds, creator, policyMarshalled, []byte(escc), []byte(vscc))
	if err != nil {
		return nil, fmt.Errorf("Error creating proposal  %s: %s", chainFuncName, err)
	}

	var signedProp *pb.SignedProposal
	signedProp, err = utils.GetSignedProposal(prop, cf.Signer)
	if err != nil {
		return nil, fmt.Errorf("Error creating signed proposal  %s: %s", chainFuncName, err)
	}

	proposalResponse, err := cf.EndorserClient.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		return nil, fmt.Errorf("Error endorsing %s: %s", chainFuncName, err)
	}

	if proposalResponse != nil {
		// assemble a signed transaction (it's an Envelope message)
		env, err := utils.CreateSignedTx(prop, cf.Signer, proposalResponse)
		if err != nil {
			return nil, fmt.Errorf("Could not assemble transaction, err %s", err)
		}

		return env, nil
	}

	return nil, nil
}

// chaincodeDeploy instantiates the chaincode. On success, the chaincode name
// (hash) is printed to STDOUT for use by subsequent chaincode-related CLI
// commands.
func chaincodeDeploy(cf *ChaincodeCmdFactory, sendInit sendInitTransaction) error {
	if channelID == "" {
		return errors.New("The required parameter 'channelID' is empty. Rerun the command with -C flag")
	}
	var err error

	defer cf.BroadcastClient.Close()

	ss := &sigSupport{cf.Signer}
	version, config, err := fetchResourceConfig(cf.EndorserClient, ss, channelID)
	if err != nil {
		return errors.Wrap(err, "failed probing channel version")
	}
	if version == v11 {
		return configBasedLifecycleUpdate(ss, cf, config, sendInit)
	}
	return sendInit()
}

func lsccInstantiate(cmd *cobra.Command, cf *ChaincodeCmdFactory) error {
	env, err := instantiate(cmd, cf)
	if err != nil {
		return err
	}
	return cf.BroadcastClient.Send(env)
}
