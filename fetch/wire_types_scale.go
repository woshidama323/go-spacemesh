// Code generated by github.com/spacemeshos/go-scale/scalegen. DO NOT EDIT.

// nolint
package fetch

import (
	"github.com/spacemeshos/go-scale"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
)

func (t *RequestMessage) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeStringWithLimit(enc, string(t.Hint), 256)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.Hash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *RequestMessage) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeStringWithLimit(dec, 256)
		if err != nil {
			return total, err
		}
		total += n
		t.Hint = datastore.Hint(field)
	}
	{
		n, err := scale.DecodeByteArray(dec, t.Hash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *ResponseMessage) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.Hash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteSliceWithLimit(enc, t.Data, 20971520)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *ResponseMessage) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.Hash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeByteSliceWithLimit(dec, 20971520)
		if err != nil {
			return total, err
		}
		total += n
		t.Data = field
	}
	return total, nil
}

func (t *RequestBatch) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.ID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.Requests, 100)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *RequestBatch) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.ID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[RequestMessage](dec, 100)
		if err != nil {
			return total, err
		}
		total += n
		t.Requests = field
	}
	return total, nil
}

func (t *ResponseBatch) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.ID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.Responses, 100)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *ResponseBatch) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.ID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[ResponseMessage](dec, 100)
		if err != nil {
			return total, err
		}
		total += n
		t.Responses = field
	}
	return total, nil
}

func (t *MeshHashRequest) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.From))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.To))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.Step))
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *MeshHashRequest) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.From = types.LayerID(field)
	}
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.To = types.LayerID(field)
	}
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Step = uint32(field)
	}
	return total, nil
}

func (t *MeshHashes) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.Hashes, 1000)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *MeshHashes) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeStructSliceWithLimit[types.Hash32](dec, 1000)
		if err != nil {
			return total, err
		}
		total += n
		t.Hashes = field
	}
	return total, nil
}

func (t *MaliciousIDs) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.NodeIDs, 100000)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *MaliciousIDs) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeStructSliceWithLimit[types.NodeID](dec, 100000)
		if err != nil {
			return total, err
		}
		total += n
		t.NodeIDs = field
	}
	return total, nil
}

func (t *EpochData) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.AtxIDs, 1000000)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *EpochData) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeStructSliceWithLimit[types.ATXID](dec, 1000000)
		if err != nil {
			return total, err
		}
		total += n
		t.AtxIDs = field
	}
	return total, nil
}

func (t *LayerData) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.Ballots, 500)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *LayerData) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeStructSliceWithLimit[types.BallotID](dec, 500)
		if err != nil {
			return total, err
		}
		total += n
		t.Ballots = field
	}
	return total, nil
}

func (t *OpinionRequest) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.Layer))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.Block)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *OpinionRequest) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Layer = types.LayerID(field)
	}
	{
		field, n, err := scale.DecodeOption[types.BlockID](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Block = field
	}
	return total, nil
}

func (t *LayerOpinion) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.PrevAggHash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.Certified)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *LayerOpinion) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.PrevAggHash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeOption[types.BlockID](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Certified = field
	}
	return total, nil
}
