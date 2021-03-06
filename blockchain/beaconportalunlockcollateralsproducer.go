package blockchain

import (
	"encoding/base64"
	"encoding/json"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"math/big"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	"strconv"
)

func buildReqUnlockOverRateCollateralsInst(
	custodianAddresStr string,
	tokenID string,
	unlockedAmounts map[string]uint64,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	unlockOverRateCollateralsContent := metadata.PortalUnlockOverRateCollateralsContent{
		CustodianAddressStr: custodianAddresStr,
		TokenID:             tokenID,
		UnlockedAmounts:     unlockedAmounts,
		TxReqID:             txReqID,
	}
	unlockOverRateCollateralsContentBytes, _ := json.Marshal(unlockOverRateCollateralsContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(unlockOverRateCollateralsContentBytes),
	}
}

type portalCusUnlockOverRateCollateralsProcessor struct {
	*portalInstProcessor
}

func (p *portalCusUnlockOverRateCollateralsProcessor) getActions() map[byte][][]string {
	return p.actions
}

func (p *portalCusUnlockOverRateCollateralsProcessor) putAction(action []string, shardID byte) {
	_, found := p.actions[shardID]
	if !found {
		p.actions[shardID] = [][]string{action}
	} else {
		p.actions[shardID] = append(p.actions[shardID], action)
	}
}

func (p *portalCusUnlockOverRateCollateralsProcessor) prepareDataBeforeProcessing(stateDB *statedb.StateDB, contentStr string) (map[string]interface{}, error) {
	return nil, nil
}

func (p *portalCusUnlockOverRateCollateralsProcessor) buildNewInsts(
	bc *BlockChain,
	contentStr string,
	shardID byte,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
	portalParams PortalParams,
	optionalData map[string]interface{},
) ([][]string, error) {
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while decoding content string of portal exchange rates action: %+v", err)
		return [][]string{}, nil
	}

	var actionData metadata.PortalUnlockOverRateCollateralsAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshal portal exchange rates action: %+v", err)
		return [][]string{}, nil
	}

	metaType := actionData.Meta.Type

	rejectInst := buildReqUnlockOverRateCollateralsInst(
		actionData.Meta.CustodianAddressStr,
		actionData.Meta.TokenID,
		map[string]uint64{},
		metaType,
		shardID,
		actionData.TxReqID,
		common.PortalCusUnlockOverRateCollateralsRejectedChainStatus,
	)
	//check key from db
	exchangeTool := NewPortalExchangeRateTool(currentPortalState.FinalExchangeRatesState, portalParams.SupportedCollateralTokens)
	custodianStateKey := statedb.GenerateCustodianStateObjectKey(actionData.Meta.CustodianAddressStr).String()
	custodianState, ok := currentPortalState.CustodianPoolState[custodianStateKey]
	if !ok || custodianState == nil {
		Logger.log.Error("ERROR: custodian not found")
		return [][]string{rejectInst}, nil
	}
	tokenAmountListInWaitingPoring := GetTotalLockedCollateralAmountInWaitingPortingsV3(currentPortalState, custodianState, actionData.Meta.TokenID)
	if (custodianState.GetLockedTokenCollaterals() == nil || custodianState.GetLockedTokenCollaterals()[actionData.Meta.TokenID] == nil) && custodianState.GetLockedAmountCollateral() == nil {
		Logger.log.Error("ERROR: custodian has no collaterals to unlock")
		return [][]string{rejectInst}, nil
	}
	if custodianState.GetHoldingPublicTokens() == nil || custodianState.GetHoldingPublicTokens()[actionData.Meta.TokenID] == 0 {
		Logger.log.Error("ERROR: custodian has no holding token to unlock")
		return [][]string{rejectInst}, nil
	}
	var lockedCollaterals map[string]uint64
	if custodianState.GetLockedTokenCollaterals() != nil && custodianState.GetLockedTokenCollaterals()[actionData.Meta.TokenID] != nil {
		lockedCollaterals = cloneMap(custodianState.GetLockedTokenCollaterals()[actionData.Meta.TokenID])
	} else {
		lockedCollaterals = make(map[string]uint64, 0)
	}
	if custodianState.GetLockedAmountCollateral() != nil {
		lockedCollaterals[common.PRVIDStr] = custodianState.GetLockedAmountCollateral()[actionData.Meta.TokenID]
	}
	totalAmountInUSD := uint64(0)
	for collateralID, tokenValue := range lockedCollaterals {
		if tokenValue < tokenAmountListInWaitingPoring[collateralID] {
			Logger.log.Errorf("ERROR: total %v locked less than amount lock in porting", collateralID)
			return [][]string{rejectInst}, nil
		}
		lockedCollateralExceptPorting := tokenValue - tokenAmountListInWaitingPoring[collateralID]
		// convert to usd
		pubTokenAmountInUSDT, err := exchangeTool.ConvertToUSD(collateralID, lockedCollateralExceptPorting)
		if err != nil {
			Logger.log.Errorf("Error when converting locked public token to prv: %v", err)
			return [][]string{rejectInst}, nil
		}
		totalAmountInUSD = totalAmountInUSD + pubTokenAmountInUSDT
	}

	// convert holding token to usd
	hodTokenAmountInUSDT, err := exchangeTool.ConvertToUSD(actionData.Meta.TokenID, custodianState.GetHoldingPublicTokens()[actionData.Meta.TokenID])
	if err != nil {
		Logger.log.Errorf("Error when converting holding public token to prv: %v", err)
		return [][]string{rejectInst}, nil
	}
	totalHoldAmountInUSDBigInt := new(big.Int).Mul(new(big.Int).SetUint64(hodTokenAmountInUSDT), new(big.Int).SetUint64(portalParams.MinUnlockOverRateCollaterals))
	minHoldUnlockedAmountInBigInt := new(big.Int).Div(totalHoldAmountInUSDBigInt, big.NewInt(10))
	if minHoldUnlockedAmountInBigInt.Cmp(new(big.Int).SetUint64(totalAmountInUSD)) >= 0 {
		Logger.log.Errorf("Error locked collaterals amount not enough to unlock")
		return [][]string{rejectInst}, nil
	}
	amountToUnlock := big.NewInt(0).Sub(new(big.Int).SetUint64(totalAmountInUSD), minHoldUnlockedAmountInBigInt).Uint64()
	listUnlockTokens, err := updateCustodianStateAfterReqUnlockCollateralV3(custodianState, amountToUnlock, actionData.Meta.TokenID, portalParams, currentPortalState)
	if err != nil || len(listUnlockTokens) == 0 {
		Logger.log.Errorf("Error when updateCustodianStateAfterReqUnlockCollateralV3: %v, %v", err, len(listUnlockTokens))
		return [][]string{rejectInst}, nil
	}

	inst := buildReqUnlockOverRateCollateralsInst(
		actionData.Meta.CustodianAddressStr,
		actionData.Meta.TokenID,
		listUnlockTokens,
		metaType,
		shardID,
		actionData.TxReqID,
		common.PortalCusUnlockOverRateCollateralsAcceptedChainStatus,
	)

	return [][]string{inst}, nil
}
