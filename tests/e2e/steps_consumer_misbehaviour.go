package main

import (
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	gov "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// starts a provider chain and an Opt-In consumer chain with one validator
func stepsStartChainsForConsumerMisbehaviour(consumerName string) []Step {
	s := []Step{
		{
			// Create a provider chain with two validators, where one validator holds 96% of the voting power
			// and the other validator holds 4% of the voting power.
			Action: StartChainAction{
				Chain: ChainID("provi"),
				Validators: []StartChainValidator{
					{Id: ValidatorID("alice"), Stake: 500000000, Allocation: 10000000000},
					{Id: ValidatorID("bob"), Stake: 20000000, Allocation: 10000000000},
				},
			},
			State: State{
				ChainID("provi"): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9500000000,
						ValidatorID("bob"):   9980000000,
					},
				},
			},
		},
		{
			Action: SubmitConsumerAdditionProposalAction{
				Chain:         ChainID("provi"),
				From:          ValidatorID("alice"),
				Deposit:       10000001,
				ConsumerChain: ChainID(consumerName),
				SpawnTime:     0,
				InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
				TopN:          0,
			},
			State: State{
				ChainID("provi"): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9489999999,
						ValidatorID("bob"):   9980000000,
					},
					Proposals: &map[uint]Proposal{
						1: ConsumerAdditionProposal{
							Deposit:       10000001,
							Chain:         ChainID(consumerName),
							SpawnTime:     0,
							InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
							Status:        gov.ProposalStatus_PROPOSAL_STATUS_VOTING_PERIOD.String(),
						},
					},
				},
			},
		},
		// add a consumer key before the chain starts
		// the key will be present in consumer genesis initial_val_set
		{
			Action: AssignConsumerPubKeyAction{
				Chain:          ChainID(consumerName),
				Validator:      ValidatorID("alice"),
				ConsumerPubkey: getDefaultValidators()[ValidatorID("alice")].ConsumerValPubKey,
				// consumer chain has not started
				// we don't need to reconfigure the node
				// since it will start with consumer key
				ReconfigureNode: false,
			},
			State: State{
				ChainID(consumerName): ChainState{
					AssignedKeys: &map[ValidatorID]string{
						ValidatorID("alice"): getDefaultValidators()[ValidatorID("alice")].ConsumerValconsAddressOnProvider,
					},
					ProviderKeys: &map[ValidatorID]string{
						ValidatorID("alice"): getDefaultValidators()[ValidatorID("alice")].ValconsAddress,
					},
				},
			},
		},
		{
			Action: OptInAction{
				Chain:     ChainID(consumerName),
				Validator: ValidatorID("alice"),
			},
			State: State{},
		},
		{
			Action: VoteGovProposalAction{
				Chain:      ChainID("provi"),
				From:       []ValidatorID{ValidatorID("alice"), ValidatorID("bob")},
				Vote:       []string{"yes", "yes"},
				PropNumber: 1,
			},
			State: State{
				ChainID("provi"): ChainState{
					Proposals: &map[uint]Proposal{
						1: ConsumerAdditionProposal{
							Deposit:       10000001,
							Chain:         ChainID(consumerName),
							SpawnTime:     0,
							InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
							Status:        gov.ProposalStatus_PROPOSAL_STATUS_PASSED.String(),
						},
					},
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9500000000,
						ValidatorID("bob"):   9980000000,
					},
				},
			},
		},
		{
			// start a consumer chain using a single big validator knowing that it holds more than 2/3 of the voting power
			Action: StartConsumerChainAction{
				ConsumerChain: ChainID(consumerName),
				ProviderChain: ChainID("provi"),
				Validators: []StartChainValidator{
					{Id: ValidatorID("alice"), Stake: 500000000, Allocation: 10000000000},
				},
			},
			State: State{
				ChainID("provi"): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9500000000,
						ValidatorID("bob"):   9980000000,
					},
				},
				ChainID(consumerName): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 10000000000,
					},
				},
			},
		},
		{
			Action: AddIbcConnectionAction{
				ChainA:  ChainID(consumerName),
				ChainB:  ChainID("provi"),
				ClientA: 0,
				ClientB: 0,
			},
			State: State{},
		},
		{
			Action: AddIbcChannelAction{
				ChainA:      ChainID(consumerName),
				ChainB:      ChainID("provi"),
				ConnectionA: 0,
				PortA:       "consumer", // TODO: check port mapping
				PortB:       "provider",
				Order:       "ordered",
			},
			State: State{},
		},
		// delegate some token and relay the resulting VSC packets
		// in order to initiates the CCV channel
		{
			Action: DelegateTokensAction{
				Chain:  ChainID("provi"),
				From:   ValidatorID("alice"),
				To:     ValidatorID("alice"),
				Amount: 11000000,
			},
			State: State{
				ChainID("provi"): ChainState{
					ValPowers: &map[ValidatorID]uint{
						ValidatorID("alice"): 511,
						ValidatorID("bob"):   20,
					},
				},
				ChainID(consumerName): ChainState{
					ValPowers: &map[ValidatorID]uint{
						ValidatorID("alice"): 500,
					},
				},
			},
		},
		{
			Action: RelayPacketsAction{
				ChainA:  ChainID("provi"),
				ChainB:  ChainID(consumerName),
				Port:    "provider",
				Channel: 0,
			},
			State: State{
				ChainID(consumerName): ChainState{
					ValPowers: &map[ValidatorID]uint{
						ValidatorID("alice"): 511,
					},
				},
			},
		},
	}

	return s
}

// stepsCauseConsumerMisbehaviour causes a ICS misbehaviour by forking a consumer chain.
func stepsCauseConsumerMisbehaviour(consumerName string) []Step {
	consumerClientID := "07-tendermint-0"
	forkRelayerConfig := "/root/.hermes/config_fork.toml"
	return []Step{
		{
			// fork the consumer chain by cloning the alice validator node
			Action: ForkConsumerChainAction{
				ConsumerChain: ChainID(consumerName),
				ProviderChain: ChainID("provi"),
				Validator:     ValidatorID("alice"),
				RelayerConfig: forkRelayerConfig,
			},
			State: State{},
		},
		{
			Action: SubmitConsumerMisbehaviourAction{
				FromChain: ChainID(consumerName),
				ToChain:   ChainID("provi"),
				ClientID:  consumerClientID,
				Submitter: ValidatorID("bob"),
			},
			State: State{
				ChainID("provi"): ChainState{
					ValPowers: &map[ValidatorID]uint{
						ValidatorID("alice"): 0, // alice is jailed
						ValidatorID("bob"):   20,
					},
					StakedTokens: &map[ValidatorID]uint{
						ValidatorID("alice"): 485450000, // alice is slashed
						ValidatorID("bob"):   20000000,
					},
				},
				ChainID(consumerName): ChainState{
					ValPowers: &map[ValidatorID]uint{
						ValidatorID("alice"): 511,
						ValidatorID("bob"):   0,
					},
				},
			},
		},
		// the Hermes relayer doesn't support evidence handling for Permissionless ICS yet
		// TODO: @Simon refactor once https://github.com/informalsystems/hermes/pull/4182 is merged.
		// start relayer to detect IBC misbehaviour
		// {
		// 	Action: StartRelayerAction{},
		// 	State:  State{},
		// },
		// {
		// 	// update the fork consumer client to create a light client attack
		// 	// which should trigger a ICS misbehaviour message
		// 	Action: UpdateLightClientAction{
		// 		Chain:         ChainID(consumerName),
		// 		ClientID:      consumerClientID,
		// 		HostChain:     ChainID("provi"),
		// 		RelayerConfig: forkRelayerConfig, // this relayer config uses the "forked" consumer
		// 	},
		// 	State: State{
		// 		ChainID("provi"): ChainState{
		// 			// alice should be jailed on the provider
		// 			ValPowers: &map[ValidatorID]uint{
		// 				ValidatorID("alice"): 0,
		// 				ValidatorID("bob"):   20,
		// 			},
		// 			// "alice" should be slashed on the provider, hence representative
		// 			// power is 511000000 - 0.05 * 511000000 = 485450000
		// 			StakedTokens: &map[ValidatorID]uint{
		// 				ValidatorID("alice"): 485450000,
		// 				ValidatorID("bob"):   20000000,
		// 			},
		// 			// The consumer light client should be frozen on the provider
		// 			ClientsFrozenHeights: &map[string]clienttypes.Height{
		// 				consumerClientID: {
		// 					RevisionNumber: 0,
		// 					RevisionHeight: 1,
		// 				},
		// 			},
		// 		},
		// 		ChainID(consumerName): ChainState{
		// 			// consumer should not have learned the jailing of alice
		// 			// since its light client is frozen on the provider
		// 			ValPowers: &map[ValidatorID]uint{
		// 				ValidatorID("alice"): 511,
		// 				ValidatorID("bob"):   0,
		// 			},
		// 		},
		// 	},
		// },
		// run Hermes relayer instance to detect the ICS misbehaviour
		// and jail alice on the provider
		// {
		// 	Action: StartConsumerEvidenceDetectorAction{
		// 		Chain:     ChainID(consumerName),
		// 		Submitter: ValidatorID("bob"),
		// 	},
		// 	State: State{
		// 		ChainID("provi"): ChainState{
		// 			ValPowers: &map[ValidatorID]uint{
		// 				ValidatorID("alice"): 511,
		// 				ValidatorID("bob"):   20,
		// 			},
		// 			StakedTokens: &map[ValidatorID]uint{
		// 				ValidatorID("alice"): 511000000,
		// 				ValidatorID("bob"):   20000000,
		// 			},
		// 		},
		// 		ChainID(consumerName): ChainState{
		// 			ValPowers: &map[ValidatorID]uint{
		// 				ValidatorID("alice"): 511,
		// 				ValidatorID("bob"):   0,
		// 			},
		// 		},
		// 	},
		// },
	}
}
