package database

import (
	"errors"

	"github.com/bitly/go-simplejson"
	"github.com/iost-official/go-iost/v3/common"
)

// VoteProducerContractName name of vote producer contract
const VoteProducerContractName = "vote_producer.iost"

// VoteContractName name of common vote contract
const VoteContractName = "vote.iost"

// VoteHandler easy to get info of vote.iost and vote_producer.iost
type VoteHandler struct {
	BasicHandler
	MapHandler
}

// AccountVoteInfo ...
type AccountVoteInfo struct {
	Option       string
	Votes        *common.Decimal
	ClearedVotes *common.Decimal
}

var statusList = []string{
	"APPLY",
	"APPROVED",
	"UNAPPLY",
	"UNAPPLY_APPROVED",
}

// ProducerVoteInfo ...
type ProducerVoteInfo struct {
	Pubkey     string
	Loc        string
	URL        string
	NetID      string
	IsProducer bool
	Status     string
	Online     bool
}

// GetAccountVoteInfo ...
func (v *VoteHandler) GetAccountVoteInfo(account string) []*AccountVoteInfo {
	idVal := v.Get(VoteProducerContractName + "-voteId")
	voteID, ok := Unmarshal(idVal).(string)
	if !ok {
		return nil
	}
	userVoteVal := v.MGet(VoteContractName+"-u_"+voteID, account)
	userVoteStr, ok := Unmarshal(userVoteVal).(string)
	if !ok {
		return nil
	}
	userVote, err := simplejson.NewJson([]byte(userVoteStr))
	if err != nil {
		return nil
	}
	result := []*AccountVoteInfo{}
	for pro := range userVote.MustMap() {
		votes, err := common.NewDecimalFromString(userVote.Get(pro).GetIndex(0).MustString(), 8)
		if err != nil {
			return nil
		}
		cleared, err := common.NewDecimalFromString(userVote.Get(pro).GetIndex(2).MustString(), 8)
		if err != nil {
			return nil
		}
		result = append(result, &AccountVoteInfo{
			Option:       pro,
			Votes:        votes.Sub(cleared),
			ClearedVotes: cleared,
		})
	}
	return result
}

// GetProducerVoteInfo ...
func (v *VoteHandler) GetProducerVoteInfo(account string) (*ProducerVoteInfo, error) {
	producerInfoVal := v.MGet(VoteProducerContractName+"-producerTable", account)
	producerInfoStr, ok := Unmarshal(producerInfoVal).(string)
	if !ok {
		return nil, errors.New("can't get producer info")
	}
	producerInfo, err := simplejson.NewJson([]byte(producerInfoStr))
	if err != nil {
		return nil, err
	}

	ret := &ProducerVoteInfo{
		Pubkey:     producerInfo.Get("pubkey").MustString(),
		Loc:        producerInfo.Get("loc").MustString(),
		URL:        producerInfo.Get("url").MustString(),
		NetID:      producerInfo.Get("netId").MustString(),
		IsProducer: producerInfo.Get("isProducer").MustBool(),
		Status:     statusList[producerInfo.Get("status").MustInt(0)],
		Online:     producerInfo.Get("online").MustBool(),
	}
	return ret, nil
}

// GetProducerVotes ...
func (v *VoteHandler) GetProducerVotes(account string) (*common.Decimal, error) {
	idVal := v.Get(VoteProducerContractName + "-voteId")
	voteID, ok := Unmarshal(idVal).(string)
	if !ok {
		return nil, errors.New("vote not found")
	}
	voteInfoVal := v.MGet(VoteContractName+"-v_"+voteID, account)
	voteInfoStr, ok := Unmarshal(voteInfoVal).(string)
	if !ok {
		return nil, errors.New("can't get producer vote info")
	}
	voteInfo, err := simplejson.NewJson([]byte(voteInfoStr))
	if err != nil {
		return nil, err
	}
	return common.NewDecimalFromString(voteInfo.Get("votes").MustString("0"), 8)
}
