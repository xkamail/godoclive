package protobuf

// The types below mimic the shape of protoc-gen-go output: a handful of
// unexported bookkeeping fields (state/sizeCache/unknownFields) followed by the
// real, exported message fields carrying protobuf+json struct tags. The
// unexported fields must never appear in the generated schema.

type messageState struct{ _ int }
type sizeCache int32
type unknownFields []byte

// Ticket mimics a generated protobuf message.
type Ticket struct {
	state         messageState  `protogen:"open.v1"`
	sizeCache     sizeCache     //nolint
	unknownFields unknownFields //nolint

	Id         string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	PlayerName string `protobuf:"bytes,2,opt,name=player_name,json=playerName,proto3" json:"player_name,omitempty"`
	Score      int32  `protobuf:"varint,3,opt,name=score,proto3" json:"score,omitempty"`
}

// Match embeds a slice of pointer messages, exercising nested protobuf structs.
type Match struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	MatchId string    `protobuf:"bytes,1,opt,name=match_id,json=matchId,proto3" json:"match_id,omitempty"`
	Tickets []*Ticket `protobuf:"bytes,2,rep,name=tickets,proto3" json:"tickets,omitempty"`
}
