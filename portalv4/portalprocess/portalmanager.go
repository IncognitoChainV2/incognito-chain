package portalprocess

import (
	bMeta "github.com/incognitochain/incognito-chain/basemeta"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/portalv4"
)

type portalInstructionProcessor interface {
	GetActions() map[byte][][]string
	PutAction(action []string, shardID byte)
	// get necessary db from stateDB to verify instructions when producing new block
	PrepareDataForBlockProducer(stateDB *statedb.StateDB, contentStr string) (map[string]interface{}, error)
	// validate and create new instructions in new beacon blocks
	BuildNewInsts(
		bc bMeta.ChainRetriever,
		contentStr string,
		shardID byte,
		currentPortalV4State *CurrentPortalV4State,
		beaconHeight uint64,
		shardHeights map[byte]uint64,
		portalParams portalv4.PortalParams,
		optionalData map[string]interface{},
	) ([][]string, error)
	// process instructions that confirmed in beacon blocks
	ProcessInsts(
		stateDB *statedb.StateDB,
		beaconHeight uint64,
		instructions []string,
		currentPortalV4State *CurrentPortalV4State,
		portalParams portalv4.PortalParams,
		updatingInfoByTokenID map[common.Hash]bMeta.UpdatingInfo,
	) error
}

type portalInstProcessor struct {
	actions map[byte][][]string
}

type PortalV4Manager struct {
	PortalInstructions map[int]portalInstructionProcessor
}

func NewPortalV4Manager() *PortalV4Manager {
	portalInstProcessor := map[int]portalInstructionProcessor{
		bMeta.PortalBurnPTokenMeta: &portalUnshieldRequestProcessor{
			portalInstProcessor: &portalInstProcessor{
				actions: map[byte][][]string{},
			},
		},
		bMeta.PortalShieldingRequestMeta: &portalShieldingRequestProcessor{
			portalInstProcessor: &portalInstProcessor{
				actions: map[byte][][]string{},
			},
		},
		bMeta.PortalReplacementFeeRequestMeta: &portalReplacementFeeRequestProcessor{
			portalInstProcessor: &portalInstProcessor{
				actions: map[byte][][]string{},
			},
		},
		bMeta.PortalSubmitConfirmedTxMeta: &portalSubmitConfirmedTxProcessor{
			portalInstProcessor: &portalInstProcessor{
				actions: map[byte][][]string{},
			},
		},
	}

	return &PortalV4Manager{
		PortalInstructions: portalInstProcessor,
	}
}
