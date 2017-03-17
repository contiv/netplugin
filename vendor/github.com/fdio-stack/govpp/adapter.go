package govpp

/*
#cgo CFLAGS: -DPNG_DEBUG=1
#cgo LDFLAGS: -lpneum

#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
#include <arpa/inet.h>
#include <pneum/pneum.h>

extern void go_msg_callback(uint16_t, uint32_t, void*, size_t);

typedef struct __attribute__((__packed__)) _req_header {
    uint16_t msg_id;
    uint32_t client_index;
    uint32_t context;
} req_header_t;

typedef struct __attribute__((__packed__)) _reply_header {
    uint16_t msg_id;
    uint32_t context;
} reply_header_t;

static void
govpp_msg_callback (unsigned char *data, int size)
{
    reply_header_t *header = ((reply_header_t *)data);
    go_msg_callback(ntohs(header->msg_id), ntohl(header->context), data, size);
    pneum_free(data);
}

static void
govpp_connect()
{
    pneum_connect("govpp", NULL, govpp_msg_callback);
}

static void
govvp_disconnect()
{
    pneum_disconnect();
}

static void
govpp_send(uint32_t context, void *data, size_t size)
{
	req_header_t *header = ((req_header_t *)data);
	header->context = htonl(context);
    pneum_write(data, size);
}

static uint32_t
govpp_get_msg_index(char *name_and_crc)
{
    return pneum_get_msg_index(name_and_crc);
}
*/
import "C"

import (
	"fmt"
	"reflect"
	"unsafe"
)

type MsgRecvCbFunc func(context uint32, msgId uint16, data []byte)

type VppAdapter interface {
	Connect()
	Disconnect()
	GetMsgId(msgName string, msgCrc string) uint16
	SendMsg(clientId uint32, data []byte)
	SetMsgRecvCallback(MsgRecvCbFunc)
}

type PneumVppAdapter struct {
	callback MsgRecvCbFunc
}

var pneumVpp *PneumVppAdapter

func (a *PneumVppAdapter) Connect() {
	pneumVpp = a
	C.govpp_connect()
}

func (a *PneumVppAdapter) Disconnect() {
	C.govvp_disconnect()
}

func (a *PneumVppAdapter) GetMsgId(msgName string, msgCrc string) uint16 {
	nameAndCrc := C.CString(fmt.Sprintf("%s_%s", msgName, msgCrc))
	defer C.free(unsafe.Pointer(nameAndCrc))

	return uint16(C.govpp_get_msg_index(nameAndCrc))
}

func (a *PneumVppAdapter) SendMsg(clientId uint32, data []byte) {
	C.govpp_send(C.uint32_t(clientId), unsafe.Pointer(&data[0]), C.size_t(len(data)))
}

func (a *PneumVppAdapter) SetMsgRecvCallback(cb MsgRecvCbFunc) {
	a.callback = cb
}

//export go_msg_callback
func go_msg_callback(msgId C.uint16_t, context C.uint32_t, data unsafe.Pointer, size C.size_t) {
	// convert unsafe.Pointer to byte slice
	slice := &reflect.SliceHeader{Data: uintptr(data), Len: int(size), Cap: int(size)}
	byteArr := *(*[]byte)(unsafe.Pointer(slice))

	pneumVpp.callback(uint32(context), uint16(msgId), byteArr)
}
