package govpp

import (
	"bytes"
	"reflect"

	"github.com/fdio-stack/govpp/api"

	log "github.com/Sirupsen/logrus"
	"github.com/lunixbochs/struc"
)

type BinaryEncoder struct{}

type vppRequestHeader struct {
	VlMsgID     uint16
	ClientIndex uint32
	Context     uint32
}

type vppReplyHeader struct {
	VlMsgID uint16
	Context uint32
}

func (*BinaryEncoder) EncodeMsg(msg api.Message, msgID uint16) ([]byte, error) {
	buf := new(bytes.Buffer)

	// encode message header
	header := &vppRequestHeader{
		VlMsgID: msgID, // TODO: also fill client index and context at once
	}
	err := struc.Pack(buf, header)
	if err != nil {
		log.WithFields(log.Fields{
			"error":  err,
			"header": header,
		}).Error("Unable to encode the message header: ", err)
		return nil, err
	}

	// encode message content
	if reflect.Indirect(reflect.ValueOf(msg)).NumField() > 0 {
		err := struc.Pack(buf, msg)
		if err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"message": msg,
			}).Error("Unable to encode the message: ", err)
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (*BinaryEncoder) DecodeMsg(data []byte, msg api.Message) error {
	buf := bytes.NewReader(data)

	// decode message header
	header := &vppReplyHeader{}
	err := struc.Unpack(buf, header)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"data":  data,
		}).Error("Unable to decode header of the message.")
		return err
	}

	buf = bytes.NewReader(data[6:])

	// decode message content
	err = struc.Unpack(buf, msg)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"data":  buf,
		}).Error("Unable to decode the message.")
		return err
	}

	return nil
}
