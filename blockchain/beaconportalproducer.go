package blockchain

import (
	"encoding/base64"
	"encoding/json"
	"github.com/binance-chain/go-sdk/types/msg"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/database/lvdb"
	"github.com/incognitochain/incognito-chain/metadata"
	relaying "github.com/incognitochain/incognito-chain/relaying/bnb"
	"strconv"
)

// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildCustodianDepositInst(
	custodianAddressStr string,
	depositedAmount uint64,
	remoteAddresses map[string]string,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	custodianDepositContent := metadata.PortalCustodianDepositContent{
		IncogAddressStr: custodianAddressStr,
		RemoteAddresses: remoteAddresses,
		DepositedAmount: depositedAmount,
		TxReqID:         txReqID,
		ShardID:         shardID,
	}
	custodianDepositContentBytes, _ := json.Marshal(custodianDepositContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(custodianDepositContentBytes),
	}
}

func buildRequestPortingInst(
	metaType int,
	shardID byte,
	reqStatus string,
	uniqueRegisterId string,
	incogAddressStr string,
	pTokenId string,
	pTokenAddress string,
	registerAmount uint64,
	portingFee uint64,
	custodian map[string]lvdb.MatchingPortingCustodianDetail,
	txReqID common.Hash,
) []string {
	portingRequestContent := metadata.PortalPortingRequestContent{
		UniqueRegisterId: uniqueRegisterId,
		IncogAddressStr:  incogAddressStr,
		PTokenId:         pTokenId,
		PTokenAddress:    pTokenAddress,
		RegisterAmount:   registerAmount,
		PortingFee:       portingFee,
		Custodian:        custodian,
		TxReqID:          txReqID,
	}

	portingRequestContentBytes, _ := json.Marshal(portingRequestContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		reqStatus,
		string(portingRequestContentBytes),
	}
}

// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildReqPTokensInst(
	uniquePortingID string,
	tokenID string,
	incogAddressStr string,
	portingAmount uint64,
	portingProof string,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	reqPTokenContent := metadata.PortalRequestPTokensContent{
		UniquePortingID: uniquePortingID,
		TokenID:         tokenID,
		IncogAddressStr: incogAddressStr,
		PortingAmount:   portingAmount,
		PortingProof:    portingProof,
		TxReqID:         txReqID,
		ShardID:         shardID,
	}
	reqPTokenContentBytes, _ := json.Marshal(reqPTokenContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(reqPTokenContentBytes),
	}
}

// buildInstructionsForCustodianDeposit builds instruction for custodian deposit action
func (blockchain *BlockChain) buildInstructionsForCustodianDeposit(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalCustodianDepositAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}

	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForCustodianDeposit]: Current Portal state is null.")
		// need to refund collateral to custodian
		inst := buildCustodianDepositInst(
			actionData.Meta.IncogAddressStr,
			actionData.Meta.DepositedAmount,
			actionData.Meta.RemoteAddresses,
			actionData.Meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalCustodianDepositRefundChainStatus,
		)
		return [][]string{inst}, nil
	}
	meta := actionData.Meta

	keyCustodianState := lvdb.NewCustodianStateKey(beaconHeight, meta.IncogAddressStr)

	if currentPortalState.CustodianPoolState[keyCustodianState] == nil {
		// new custodian
		newCustodian, _ := NewCustodianState(meta.IncogAddressStr, meta.DepositedAmount, meta.DepositedAmount, nil, nil, meta.RemoteAddresses)
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	} else {
		// custodian deposited before
		// update state of the custodian
		custodian := currentPortalState.CustodianPoolState[keyCustodianState]
		totalCollateral := custodian.TotalCollateral + meta.DepositedAmount
		freeCollateral := custodian.FreeCollateral + meta.DepositedAmount
		holdingPubTokens := custodian.HoldingPubTokens
		lockedAmountCollateral := custodian.LockedAmountCollateral
		remoteAddresses := custodian.RemoteAddresses
		for tokenSymbol, address := range meta.RemoteAddresses {
			if remoteAddresses[tokenSymbol] == "" {
				remoteAddresses[tokenSymbol] = address
			}
		}

		newCustodian, _ := NewCustodianState(meta.IncogAddressStr, totalCollateral, freeCollateral, holdingPubTokens, lockedAmountCollateral, remoteAddresses)
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	}

	inst := buildCustodianDepositInst(
		actionData.Meta.IncogAddressStr,
		actionData.Meta.DepositedAmount,
		actionData.Meta.RemoteAddresses,
		actionData.Meta.Type,
		shardID,
		actionData.TxReqID,
		common.PortalCustodianDepositAcceptedChainStatus,
	)
	return [][]string{inst}, nil
}

func (blockchain *BlockChain) buildInstructionsForPortingRequest(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("Porting request: an error occurred while decoding content string of portal porting request action: %+v", err)
		return [][]string{}, nil
	}

	var actionData metadata.PortalUserRegisterAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("Porting request: an error occurred while unmarshal portal porting request action: %+v", err)
		return [][]string{}, nil
	}

	if currentPortalState == nil {
		Logger.log.Warn("Porting request: Current Portal state is null")
		return [][]string{}, nil
	}

	db := blockchain.GetDatabase()


	keyPortingRequest := lvdb.NewPortingRequestKey(actionData.Meta.UniqueRegisterId)
	//check unique id form temp
	if _, ok := currentPortalState.PortingIdRequests[keyPortingRequest]; ok {
		Logger.log.Errorf("Porting request: Porting request id exist from temp data, key %v", keyPortingRequest)
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//check unique id from record from db
	portingRequestKeyExist, err := db.GetItemPortalByKey([]byte(keyPortingRequest))

	if err != nil {
		Logger.log.Errorf("Porting request: Get item portal by prefix error: %+v", err)

		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	if portingRequestKeyExist != nil {
		Logger.log.Errorf("Porting request: Porting request exist, key %v", keyPortingRequest)
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	waitingPortingRequestKey := lvdb.NewWaitingPortingReqKey(beaconHeight, actionData.Meta.UniqueRegisterId)
	if _, ok := currentPortalState.WaitingPortingRequests[waitingPortingRequestKey]; ok {
		Logger.log.Errorf("Porting request: Waiting porting request exist, key %v", waitingPortingRequestKey)
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//get exchange rates
	exchangeRatesKey := lvdb.NewFinalExchangeRatesKey(beaconHeight)
	exchangeRatesState, ok := currentPortalState.FinalExchangeRates[exchangeRatesKey]
	if !ok {
		Logger.log.Errorf("Porting request, exchange rates not found")
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//todo: create error instruction
	if currentPortalState.CustodianPoolState == nil {
		Logger.log.Errorf("Porting request: Custodian not found")
		return [][]string{}, nil
	}

	var sortCustodianStateByFreeCollateral []CustodianStateSlice
	err = sortCustodianByAmountAscent(actionData.Meta, currentPortalState.CustodianPoolState, &sortCustodianStateByFreeCollateral)

	if err != nil {
		return [][]string{}, nil
	}

	if len(sortCustodianStateByFreeCollateral) <= 0 {
		Logger.log.Errorf("Porting request, custodian not found")

		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//pick one
	pickCustodianResult, _ := pickSingleCustodian(actionData.Meta, exchangeRatesState, sortCustodianStateByFreeCollateral)

	Logger.log.Infof("Porting request, pick single custodian result %v", len(pickCustodianResult))
	//pick multiple
	if len(pickCustodianResult) == 0 {
		pickCustodianResult, _ = pickMultipleCustodian(actionData.Meta, exchangeRatesState, sortCustodianStateByFreeCollateral)
		Logger.log.Infof("Porting request, pick multiple custodian result %v", len(pickCustodianResult))
	}
	//end
	if len(pickCustodianResult) == 0 {
		Logger.log.Errorf("Porting request, custodian not found")
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			pickCustodianResult,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//validation porting fees
	pToken2PRV := exchangeRatesState.ExchangePToken2PRVByTokenId(actionData.Meta.PTokenId, actionData.Meta.RegisterAmount)
	exchangePortingFees := CalculatePortingFees(pToken2PRV)
	Logger.log.Infof("Porting request, porting fees need %v", exchangePortingFees)

	if actionData.Meta.PortingFee < exchangePortingFees {
		Logger.log.Errorf("Porting request, Porting fees is wrong")

		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.PTokenAddress,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			pickCustodianResult,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	inst := buildRequestPortingInst(
		actionData.Meta.Type,
		shardID,
		common.PortalPortingRequestAcceptedStatus,
		actionData.Meta.UniqueRegisterId,
		actionData.Meta.IncogAddressStr,
		actionData.Meta.PTokenId,
		actionData.Meta.PTokenAddress,
		actionData.Meta.RegisterAmount,
		actionData.Meta.PortingFee,
		pickCustodianResult,
		actionData.TxReqID,
	) //return  metadata.PortalPortingRequestContent at instruct[3]

	//store porting request id for validation next instruct
	currentPortalState.PortingIdRequests[keyPortingRequest] = keyPortingRequest
	return [][]string{inst}, nil
}

// buildInstructionsForCustodianDeposit builds instruction for custodian deposit action
func (blockchain *BlockChain) buildInstructionsForReqPTokens(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {

	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalRequestPTokensAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}
	meta := actionData.Meta

	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForCustodianDeposit]: Current Portal state is null.")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check meta.UniquePortingID is in waiting PortingRequests list in portal state or not
	portingID := meta.UniquePortingID
	keyWaitingPortingRequest := lvdb.NewWaitingPortingReqKey(beaconHeight, portingID)
	waitingPortingRequest := currentPortalState.WaitingPortingRequests[keyWaitingPortingRequest]
	if waitingPortingRequest == nil {
		Logger.log.Errorf("PortingID is not existed in waiting porting requests list")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}
	db := blockchain.GetDatabase()

	// check porting request status of portingID from db
	portingReqStatus, err := db.GetPortingRequestStatusByPortingID(meta.UniquePortingID)
	if err != nil {
		Logger.log.Errorf("Can not get porting req status for portingID %v, %v\n", meta.UniquePortingID, err)
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	if portingReqStatus != common.PortalPortingReqWaitingStatus {
		Logger.log.Errorf("PortingID status invalid")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check tokenID
	if meta.TokenID != metadata.PortalSupportedTokenMap[waitingPortingRequest.TokenID] {
		Logger.log.Errorf("TokenID is not correct in portingID req")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check porting amount
	if meta.PortingAmount != waitingPortingRequest.Amount {
		Logger.log.Errorf("PortingAmount is not correct in portingID req")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	if meta.TokenID == metadata.PortalSupportedTokenMap[metadata.PortalTokenSymbolBTC] {
		//todo:
	} else if meta.TokenID == metadata.PortalSupportedTokenMap[metadata.PortalTokenSymbolBNB] {
		// parse PortingProof in meta
		txProofBNB, err := relaying.ParseBNBProofFromB64EncodeJsonStr(meta.PortingProof)
		if err != nil {
			Logger.log.Errorf("PortingProof is invalid %v\n", err)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		isValid, err := txProofBNB.Verify(db)
		if !isValid || err != nil {
			Logger.log.Errorf("Verify txProofBNB failed %v", err)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// parse Tx from Data in txProofBNB
		txBNB, err := relaying.ParseTxFromData(txProofBNB.Proof.Data)
		if err != nil {
			Logger.log.Errorf("Data in PortingProof is invalid %v", err)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check memo attach portingID req:
		type PortingMemoBNB struct {
			PortingID string `json:"PortingID"`
		}
		memo := txBNB.Memo
		Logger.log.Infof("[buildInstructionsForReqPTokens] memo: %v\n", memo)
		memoBytes, err2 := base64.StdEncoding.DecodeString(memo)
		if err2 != nil {
			Logger.log.Errorf("Can not decode memo in tx bnb proof", err2)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}
		Logger.log.Infof("[buildInstructionsForReqPTokens] memoBytes: %v\n", memoBytes)

		var portingMemo PortingMemoBNB
		err2 = json.Unmarshal(memoBytes, &portingMemo)
		if err2 != nil {
			Logger.log.Errorf("Can not unmarshal memo in tx bnb proof", err2)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		if portingMemo.PortingID != meta.UniquePortingID {
			Logger.log.Errorf("PortingId in memoTx is not matched with portingID in metadata", err2)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check whether amount transfer in txBNB is equal porting amount or not
		// check receiver and amount in tx
		// get list matching custodians in waitingPortingRequest
		custodians := waitingPortingRequest.Custodians

		outputs := txBNB.Msgs[0].(msg.SendMsg).Outputs

		for _, cusDetail := range custodians {
			remoteAddressNeedToBeTransfer := cusDetail.RemoteAddress
			amountNeedToBeTransfer := cusDetail.Amount

			for _, out := range outputs {
				addr := string(out.Address)
				if addr != remoteAddressNeedToBeTransfer {
					continue
				}

				// calculate amount that was transferred to custodian's remote address
				amountTransfer := int64(0)
				for _, coin := range out.Coins {
					if coin.Denom == relaying.DenomBNB {
						amountTransfer += coin.Amount
						// note: log error for debug
						Logger.log.Errorf("TxProof-BNB coin.Amount %d",
							coin.Amount)
					}
				}
				if convertExternalBNBAmountToIncAmount(amountTransfer) != int64(amountNeedToBeTransfer) {
					Logger.log.Errorf("TxProof-BNB is invalid - Amount transfer to %s must be equal %d, but got %d",
						addr, amountNeedToBeTransfer, amountTransfer)
					inst := buildReqPTokensInst(
						meta.UniquePortingID,
						meta.TokenID,
						meta.IncogAddressStr,
						meta.PortingAmount,
						meta.PortingProof,
						meta.Type,
						shardID,
						actionData.TxReqID,
						common.PortalReqPTokensRejectedChainStatus,
					)
					return [][]string{inst}, nil
				}
			}
		}

		inst := buildReqPTokensInst(
			actionData.Meta.UniquePortingID,
			actionData.Meta.TokenID,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PortingAmount,
			actionData.Meta.PortingProof,
			actionData.Meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensAcceptedChainStatus,
		)

		// remove waiting porting request from currentPortalState
		removeWaitingPortingReqByKey(keyWaitingPortingRequest, currentPortalState)
		return [][]string{inst}, nil
	} else {
		Logger.log.Errorf("TokenID is not supported currently on Portal")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	return [][]string{}, nil
}

func (blockchain *BlockChain) buildInstructionsForExchangeRates(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while decoding content string of portal exchange rates action: %+v", err)
		return [][]string{}, nil
	}

	var actionData metadata.PortalExchangeRatesAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshal portal exchange rates action: %+v", err)
		return [][]string{}, nil
	}

	exchangeRatesKey := lvdb.NewExchangeRatesRequestKey(
		beaconHeight+1,
		actionData.TxReqID.String(),
	)

	db := blockchain.GetDatabase()
	//check key from db
	exchangeRatesKeyExist, err := db.GetItemPortalByKey([]byte(exchangeRatesKey))

	if err != nil {
		Logger.log.Errorf("ERROR: Get exchange rates error: %+v", err)

		portalExchangeRatesContent := metadata.PortalExchangeRatesContent{
			SenderAddress:   actionData.Meta.SenderAddress,
			Rates:           actionData.Meta.Rates,
			TxReqID:         actionData.TxReqID,
			LockTime:        actionData.LockTime,
			UniqueRequestId: exchangeRatesKey,
		}

		portalExchangeRatesContentBytes, _ := json.Marshal(portalExchangeRatesContent)

		inst := []string{
			strconv.Itoa(metaType),
			strconv.Itoa(int(shardID)),
			common.PortalExchangeRatesRejectedStatus,
			string(portalExchangeRatesContentBytes),
		}

		return [][]string{inst}, nil
	}

	if exchangeRatesKeyExist != nil {
		Logger.log.Errorf("ERROR: exchange rates key is duplicated")

		portalExchangeRatesContent := metadata.PortalExchangeRatesContent{
			SenderAddress:   actionData.Meta.SenderAddress,
			Rates:           actionData.Meta.Rates,
			TxReqID:         actionData.TxReqID,
			LockTime:        actionData.LockTime,
			UniqueRequestId: exchangeRatesKey,
		}

		portalExchangeRatesContentBytes, _ := json.Marshal(portalExchangeRatesContent)

		inst := []string{
			strconv.Itoa(metaType),
			strconv.Itoa(int(shardID)),
			common.PortalExchangeRatesRejectedStatus,
			string(portalExchangeRatesContentBytes),
		}

		return [][]string{inst}, nil
	}

	//success
	portalExchangeRatesContent := metadata.PortalExchangeRatesContent{
		SenderAddress:   actionData.Meta.SenderAddress,
		Rates:           actionData.Meta.Rates,
		TxReqID:         actionData.TxReqID,
		LockTime:        actionData.LockTime,
		UniqueRequestId: exchangeRatesKey,
	}

	portalExchangeRatesContentBytes, _ := json.Marshal(portalExchangeRatesContent)

	inst := []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		common.PortalExchangeRatesSuccessStatus,
		string(portalExchangeRatesContentBytes),
	}

	return [][]string{inst}, nil
}

// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildRedeemRequestInst(
	uniqueRedeemID string,
	tokenID string,
	redeemAmount uint64,
	incAddressStr string,
	remoteAddress string,
	redeemFee uint64,
	matchingCustodianDetail map[string]*lvdb.MatchingRedeemCustodianDetail,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	redeemRequestContent := metadata.PortalRedeemRequestContent{
		UniqueRedeemID:          uniqueRedeemID,
		TokenID:                 tokenID,
		RedeemAmount:            redeemAmount,
		IncAddressStr:           incAddressStr,
		RemoteAddress:           remoteAddress,
		MatchingCustodianDetail: matchingCustodianDetail,
		RedeemFee:               redeemFee,
		TxReqID:                 txReqID,
		ShardID:                 shardID,
	}
	redeemRequestContentBytes, _ := json.Marshal(redeemRequestContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(redeemRequestContentBytes),
	}
}

// buildInstructionsForRedeemRequest builds instruction for redeem request action
func (blockchain *BlockChain) buildInstructionsForRedeemRequest(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal redeem request action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalRedeemRequestAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal redeem request action: %+v", err)
		return [][]string{}, nil
	}

	meta := actionData.Meta
	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForRedeemRequest]: Current Portal state is null.")
		// need to mint ptoken to user
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.IncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	redeemID := meta.UniqueRedeemID

	// check uniqueRedeemID is existed waitingRedeem list or not
	keyWaitingRedeemRequest := lvdb.NewWaitingRedeemReqKey(beaconHeight, redeemID)
	waitingRedeemRequest := currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest]
	if waitingRedeemRequest != nil {
		Logger.log.Errorf("RedeemID is existed in waiting redeem requests list %v\n", redeemID)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.IncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	db := blockchain.GetDatabase()

	// check uniqueRedeemID is existed in db or not
	redeemRequestBytes, err := db.GetRedeemRequestByRedeemID(meta.UniqueRedeemID)
	if err != nil {
		Logger.log.Errorf("Can not get redeem req status for redeemID %v, %v\n", meta.UniqueRedeemID, err)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.IncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	} else if len(redeemRequestBytes) > 0 {
		Logger.log.Errorf("RedeemID is existed in redeem requests list in db %v\n", redeemID)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.IncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// get tokenSymbol from redeemTokenID
	tokenSymbol := ""
	for tokenSym, incTokenID := range metadata.PortalSupportedTokenMap {
		if incTokenID == meta.TokenID {
			tokenSymbol = tokenSym
			break
		}
	}

	// pick custodian(s) who holding public token to return user
	matchingCustodiansDetail, err := pickupCustodianForRedeem(meta.RedeemAmount, tokenSymbol, currentPortalState)
	if err != nil {
		Logger.log.Errorf("Error when pick up custodian for redeem %v\n", err)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.IncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// add to waiting Redeem list
	redeemRequest, _ := NewRedeemRequestState(
		meta.UniqueRedeemID,
		actionData.TxReqID,
		meta.TokenID,
		meta.IncAddressStr,
		meta.RemoteAddress,
		meta.RedeemAmount,
		matchingCustodiansDetail,
		meta.RedeemFee,
		beaconHeight + 1,
	)
	currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest] = redeemRequest

	// update custodian state (holding public tokens)
	for k, cus := range matchingCustodiansDetail {
		if currentPortalState.CustodianPoolState[k].HoldingPubTokens[tokenSymbol] < cus.Amount {
			Logger.log.Errorf("Amount holding public tokens is less than matching redeem amount")
			inst := buildRedeemRequestInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.RedeemAmount,
				meta.IncAddressStr,
				meta.RemoteAddress,
				meta.RedeemFee,
				nil,
				meta.Type,
				actionData.ShardID,
				actionData.TxReqID,
				common.PortalRedeemRequestRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}
		currentPortalState.CustodianPoolState[k].HoldingPubTokens[tokenSymbol] -= cus.Amount
	}

	Logger.log.Infof("[Portal] Build accepted instruction for redeem request")
	inst := buildRedeemRequestInst(
		meta.UniqueRedeemID,
		meta.TokenID,
		meta.RedeemAmount,
		meta.IncAddressStr,
		meta.RemoteAddress,
		meta.RedeemFee,
		matchingCustodiansDetail,
		meta.Type,
		actionData.ShardID,
		actionData.TxReqID,
		common.PortalRedeemRequestAcceptedChainStatus,
	)
	return [][]string{inst}, nil
}

// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildReqUnlockCollateralInst(
	uniqueRedeemID string,
	tokenID string,
	custodianAddressStr string,
	redeemAmount uint64,
	unlockAmount uint64,
	redeemProof string,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	reqUnlockCollateralContent := metadata.PortalRequestUnlockCollateralContent{
		UniqueRedeemID:      uniqueRedeemID,
		TokenID:             tokenID,
		CustodianAddressStr: custodianAddressStr,
		RedeemAmount:        redeemAmount,
		UnlockAmount: unlockAmount,
		RedeemProof:         redeemProof,
		TxReqID:             txReqID,
		ShardID:             shardID,
	}
	reqUnlockCollateralContentBytes, _ := json.Marshal(reqUnlockCollateralContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(reqUnlockCollateralContentBytes),
	}
}

// buildInstructionsForReqUnlockCollateral builds instruction for custodian deposit action
func (blockchain *BlockChain) buildInstructionsForReqUnlockCollateral(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {

	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal request unlock collateral action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalRequestUnlockCollateralAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal request unlock collateral action: %+v", err)
		return [][]string{}, nil
	}
	meta := actionData.Meta

	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForReqUnlockCollateral]: Current Portal state is null.")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check meta.UniqueRedeemID is in waiting RedeemRequests list in portal state or not
	redeemID := meta.UniqueRedeemID
	keyWaitingRedeemRequest := lvdb.NewWaitingRedeemReqKey(beaconHeight, redeemID)
	waitingRedeemRequest := currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest]
	if waitingRedeemRequest == nil {
		Logger.log.Errorf("redeemID is not existed in waiting redeem requests list")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}
	db := blockchain.GetDatabase()

	// check status of request unlock collateral by redeemID
	redeemReqStatusBytes, err := db.GetRedeemRequestByRedeemID(redeemID)
	if err != nil {
		Logger.log.Errorf("Can not get redeem request by redeemID from db %v\n", err)
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}
	var redeemRequest metadata.PortalRedeemRequestStatus
	err = json.Unmarshal(redeemReqStatusBytes, &redeemRequest)
	if err != nil {
		Logger.log.Errorf("Can not unmarshal redeem request %v\n", err)
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	if redeemRequest.Status != common.PortalRedeemReqWaitingStatus {
		Logger.log.Errorf("Redeem request %v has invalid status %v\n", redeemID, redeemRequest.Status)
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check tokenID
	if meta.TokenID != waitingRedeemRequest.TokenID {
		Logger.log.Errorf("TokenID is not correct in redeemID req")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check redeem amount of matching custodian
	if meta.RedeemAmount != waitingRedeemRequest.Custodians[meta.CustodianAddressStr].Amount {
		Logger.log.Errorf("RedeemAmount is not correct in redeemID req")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// validate proof and memo in tx
	if meta.TokenID == metadata.PortalSupportedTokenMap[metadata.PortalTokenSymbolBTC] {
		//todo:
	} else if meta.TokenID == metadata.PortalSupportedTokenMap[metadata.PortalTokenSymbolBNB] {
		// parse PortingProof in meta
		txProofBNB, err := relaying.ParseBNBProofFromB64EncodeJsonStr(meta.RedeemProof)
		if err != nil {
			Logger.log.Errorf("RedeemProof is invalid %v\n", err)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		isValid, err := txProofBNB.Verify(db)
		if !isValid || err != nil {
			Logger.log.Errorf("Verify txProofBNB failed %v", err)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// parse Tx from Data in txProofBNB
		txBNB, err := relaying.ParseTxFromData(txProofBNB.Proof.Data)
		if err != nil {
			Logger.log.Errorf("Data in RedeemProof is invalid %v", err)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check memo attach redeemID req:
		type RedeemMemoBNB struct {
			RedeemID string `json:"RedeemID"`
		}
		memo := txBNB.Memo
		Logger.log.Infof("[buildInstructionsForReqUnlockCollateral] memo: %v\n", memo)
		memoBytes, err2 := base64.StdEncoding.DecodeString(memo)
		if err2 != nil {
			Logger.log.Errorf("Can not decode memo in tx bnb proof", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}
		Logger.log.Infof("[buildInstructionsForReqUnlockCollateral] memoBytes: %v\n", memoBytes)

		var redeemMemo RedeemMemoBNB
		err2 = json.Unmarshal(memoBytes, &redeemMemo)
		if err2 != nil {
			Logger.log.Errorf("Can not unmarshal memo in tx bnb proof", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		if redeemMemo.RedeemID != meta.UniqueRedeemID {
			Logger.log.Errorf("PortingId in memoTx is not matched with redeemID in metadata", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check whether amount transfer in txBNB is equal redeem amount or not
		// check receiver and amount in tx
		// get list matching custodians in waitingRedeemRequest
		//custodians := waitingRedeemRequest.Custodians

		outputs := txBNB.Msgs[0].(msg.SendMsg).Outputs

		remoteAddressNeedToBeTransfer := waitingRedeemRequest.RedeemerRemoteAddress
		amountNeedToBeTransfer := meta.RedeemAmount

		for _, out := range outputs {
			addr := string(out.Address)
			if addr != remoteAddressNeedToBeTransfer {
				continue
			}

			// calculate amount that was transferred to custodian's remote address
			amountTransfer := int64(0)
			for _, coin := range out.Coins {
				if coin.Denom == relaying.DenomBNB {
					amountTransfer += coin.Amount
					// note: log error for debug
					Logger.log.Errorf("TxProof-BNB coin.Amount %d",
						coin.Amount)
				}
			}
			if convertExternalBNBAmountToIncAmount(amountTransfer) != int64(amountNeedToBeTransfer) {
				Logger.log.Errorf("TxProof-BNB is invalid - Amount transfer to %s must be equal %d, but got %d",
					addr, amountNeedToBeTransfer, amountTransfer)
				inst := buildReqUnlockCollateralInst(
					meta.UniqueRedeemID,
					meta.TokenID,
					meta.CustodianAddressStr,
					meta.RedeemAmount,
					0,
					meta.RedeemProof,
					meta.Type,
					shardID,
					actionData.TxReqID,
					common.PortalReqUnlockCollateralRejectedChainStatus,
				)
				return [][]string{inst}, nil
			}
		}
		// get tokenSymbol from redeemTokenID
		tokenSymbol := ""
		for tokenSym, incTokenID := range metadata.PortalSupportedTokenMap {
			if incTokenID == meta.TokenID {
				tokenSymbol = tokenSym
				break
			}
		}

		// update custodian state (FreeCollateral, LockedAmountCollateral)
		custodianStateKey := lvdb.NewCustodianStateKey(beaconHeight, meta.CustodianAddressStr)
		finalExchangeRateKey := lvdb.NewFinalExchangeRatesKey(beaconHeight)
		unlockAmount, err2 := updateFreeCollateralCustodian(
			currentPortalState.CustodianPoolState[custodianStateKey],
			meta.RedeemAmount, tokenSymbol,
			currentPortalState.FinalExchangeRates[finalExchangeRateKey])
		if err2 != nil {
			Logger.log.Errorf("Error when update free collateral amount for custodian", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// update redeem request state in WaitingRedeemRequest (remove custodian from matchingCustodianDetail)
		delete(currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest].Custodians, meta.CustodianAddressStr)

		// remove redeem request from WaitingRedeemRequest list when all matching custodians return public token to user
		// when list matchingCustodianDetail is empty
		if len(currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest].Custodians) == 0 {
			delete(currentPortalState.WaitingRedeemRequests, keyWaitingRedeemRequest)
		}

		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			unlockAmount,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralAcceptedChainStatus,
		)

		return [][]string{inst}, nil
	} else {
		Logger.log.Errorf("TokenID is not supported currently on Portal")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	return [][]string{}, nil
}
