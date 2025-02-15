package salunamedia

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prebid/openrtb/v19/openrtb2"

	"github.com/prebid/prebid-server/v2/adapters"
	"github.com/prebid/prebid-server/v2/config"
	"github.com/prebid/prebid-server/v2/errortypes"
	"github.com/prebid/prebid-server/v2/openrtb_ext"
)

type adapter struct {
	endpoint string
}

type bidExt struct {
	MediaType string `json:"mediaType"`
}

func Builder(bidderName openrtb_ext.BidderName, config config.Adapter, server config.Server) (adapters.Bidder, error) {
	bidder := &adapter{
		endpoint: config.Endpoint,
	}
	return bidder, nil
}

func (a *adapter) MakeRequests(
	openRTBRequest *openrtb2.BidRequest,
	reqInfo *adapters.ExtraRequestInfo,
) (
	requestsToBidder []*adapters.RequestData,
	errs []error,
) {

	reqJSON, err := json.Marshal(openRTBRequest)
	if err != nil {
		return nil, []error{err}
	}

	headers := http.Header{}
	headers.Add("Content-Type", "application/json;charset=utf-8")
	headers.Add("Accept", "application/json")

	return []*adapters.RequestData{{
		Method:  http.MethodPost,
		Body:    reqJSON,
		Uri:     a.endpoint,
		Headers: headers,
	}}, nil
}

func (a *adapter) MakeBids(
	openRTBRequest *openrtb2.BidRequest,
	requestToBidder *adapters.RequestData,
	bidderRawResponse *adapters.ResponseData,
) (
	bidderResponse *adapters.BidderResponse,
	errs []error,
) {
	if bidderRawResponse.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if bidderRawResponse.StatusCode == http.StatusBadRequest {
		return nil, []error{&errortypes.BadInput{
			Message: fmt.Sprintf("Bad Request. %s", string(bidderRawResponse.Body)),
		}}
	}

	if bidderRawResponse.StatusCode == http.StatusServiceUnavailable {
		return nil, []error{&errortypes.BadInput{
			Message: "Bidder unavailable. Please contact the bidder support.",
		}}
	}

	if bidderRawResponse.StatusCode != http.StatusOK {
		return nil, []error{&errortypes.BadServerResponse{
			Message: fmt.Sprintf("Status Code: [ %d ] %s", bidderRawResponse.StatusCode, string(bidderRawResponse.Body)),
		}}
	}

	responseBody := bidderRawResponse.Body
	var bidResp openrtb2.BidResponse
	if err := json.Unmarshal(responseBody, &bidResp); err != nil {
		return nil, []error{err}
	}

	if len(bidResp.SeatBid) == 0 {
		return nil, []error{&errortypes.BadServerResponse{
			Message: "Empty SeatBid",
		}}
	}

	bidResponse := adapters.NewBidderResponseWithBidsCapacity(1)

	bids := bidResp.SeatBid[0].Bid

	if len(bids) == 0 {
		return nil, []error{&errortypes.BadServerResponse{
			Message: "Empty SeatBid.Bids",
		}}
	}

	bid := bids[0]

	var bidExt bidExt
	var bidType openrtb_ext.BidType

	if err := json.Unmarshal(bid.Ext, &bidExt); err != nil {
		return nil, []error{&errortypes.BadServerResponse{
			Message: "Missing BidExt",
		}}
	}

	bidType, err := getBidType(bidExt)

	if err != nil {
		return nil, []error{err}
	}

	bidResponse.Bids = append(bidResponse.Bids, &adapters.TypedBid{
		Bid:     &bid,
		BidType: bidType,
	})
	return bidResponse, nil
}

func getBidType(ext bidExt) (openrtb_ext.BidType, error) {
	return openrtb_ext.ParseBidType(ext.MediaType)
}
