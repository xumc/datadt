package tcpmonitor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/xumc/datadt/display"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/google/gopacket"
	"time"
)

const (
	ComQueryRequestPacket string     = "【Query】"
	OkPacket string                  = "【Ok】"
	ErrorPacket string               = "【Err】"
	PreparePacket string             = "【Pretreatment】"
	SendClientHandshakePacket string = "【User Auth】"
	SendServerHandshakePacket string = "【Login】"

	COM_SLEEP byte          = 0
	COM_QUIT                = 1
	COM_INIT_DB             = 2
	COM_QUERY               = 3
	COM_FIELD_LIST          = 4
	COM_CREATE_DB           = 5
	COM_DROP_DB             = 6
	COM_REFRESH             = 7
	COM_SHUTDOWN            = 8
	COM_STATISTICS          = 9
	COM_PROCESS_INFO        = 10
	COM_CONNECT             = 11
	COM_PROCESS_KILL        = 12
	COM_DEBUG               = 13
	COM_PING                = 14
	COM_TIME                = 15
	COM_DELAYED_INSERT      = 16
	COM_CHANGE_USER         = 17
	COM_BINLOG_DUMP         = 18
	COM_TABLE_DUMP          = 19
	COM_CONNECT_OUT         = 20
	COM_REGISTER_SLAVE      = 21
	COM_STMT_PREPARE        = 22
	COM_STMT_EXECUTE        = 23
	COM_STMT_SEND_LONG_DATA = 24
	COM_STMT_CLOSE          = 25
	COM_STMT_RESET          = 26
	COM_SET_OPTION          = 27
	COM_STMT_FETCH          = 28
	COM_DAEMON              = 29
	COM_BINLOG_DUMP_GTID    = 30
	COM_RESET_CONNECTION    = 31

	MYSQL_TYPE_DECIMAL byte = 0
	MYSQL_TYPE_TINY         = 1
	MYSQL_TYPE_SHORT        = 2
	MYSQL_TYPE_LONG         = 3
	MYSQL_TYPE_FLOAT        = 4
	MYSQL_TYPE_DOUBLE       = 5
	MYSQL_TYPE_NULL         = 6
	MYSQL_TYPE_TIMESTAMP    = 7
	MYSQL_TYPE_LONGLONG     = 8
	MYSQL_TYPE_INT24        = 9
	MYSQL_TYPE_DATE         = 10
	MYSQL_TYPE_TIME         = 11
	MYSQL_TYPE_DATETIME     = 12
	MYSQL_TYPE_YEAR         = 13
	MYSQL_TYPE_NEWDATE      = 14
	MYSQL_TYPE_VARCHAR      = 15
	MYSQL_TYPE_BIT          = 16
)

const (
	MYSQL_TYPE_JSON byte = iota + 0xf5
	MYSQL_TYPE_NEWDECIMAL
	MYSQL_TYPE_ENUM
	MYSQL_TYPE_SET
	MYSQL_TYPE_TINY_BLOB
	MYSQL_TYPE_MEDIUM_BLOB
	MYSQL_TYPE_LONG_BLOB
	MYSQL_TYPE_BLOB
	MYSQL_TYPE_VAR_STRING
	MYSQL_TYPE_STRING
	MYSQL_TYPE_GEOMETRY
)

type Mysql struct{
	TcpCommon
	source     map[string]*connStream
}

func NewMysql(tc TcpCommon) *Mysql {
	return &Mysql{
		TcpCommon: tc,
		source: make(map[string]*connStream),
	}
}

type connStream struct {
	packets  chan *packet
	stmtMap  map[uint32]*Stmt
	outputer display.Outputer
}

type packet struct {
	isClientFlow bool
	seq        int
	length     int
	payload   []byte
}

func (m *Mysql) Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer) {
	uuid := fmt.Sprintf("%v:%v", net.FastHash(), transport.FastHash())

	//generate resolve's stream
	if _, ok := m.source[uuid]; !ok {

		var newStream = connStream{
			packets:make(chan *packet, 100),
			stmtMap:make(map[uint32]*Stmt),
			outputer: outputer,
		}

		m.source[uuid] = &newStream
		go newStream.resolve()
	}

	for {

		newPacket := m.newPacket(net, transport, buf)

		if newPacket == nil {
			return
		}

		m.source[uuid].packets <- newPacket
	}

}

func (m *Mysql) newPacket(net, transport gopacket.Flow, r io.Reader) *packet {

	//read packet
	var payload bytes.Buffer
	var seq uint8
	var err error
	if seq, err = m.resolvePacketTo(r, &payload); err != nil {
		return nil
	}

	//close stream
	if err == io.EOF {
		fmt.Println(net, transport, " close")
		return nil
	} else if err != nil {
		fmt.Println("ERR : Unknown stream", net, transport, ":", err)
	}

	//generate new packet
	var pk = packet{
		seq: int(seq),
		length:payload.Len(),
		payload:payload.Bytes(),
	}

	if transport.Src().String() == strconv.FormatUint(uint64(m.Port), 10) {
		pk.isClientFlow = false
	}else{
		pk.isClientFlow = true
	}

	return &pk
}

func (m *Mysql) resolvePacketTo(r io.Reader, w io.Writer) (uint8, error) {

	header := make([]byte, 4)
	if n, err := io.ReadFull(r, header); err != nil {
		if n == 0 && err == io.EOF {
			return 0, io.EOF
		}
		return 0, errors.New("ERR : Unknown stream")
	}

	length := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)

	var seq uint8
	seq = header[3]

	if n, err := io.CopyN(w, r, int64(length)); err != nil {
		return 0, errors.New("ERR : Unknown stream")
	} else if n != int64(length) {
		return 0, errors.New("ERR : Unknown stream")
	} else {
		return seq, nil
	}

	return seq, nil
}

func (stm *connStream) resolve() {
	for {
		select {
		case packet := <- stm.packets:
			if packet.length != 0 {
				if packet.isClientFlow {
					stm.resolveClientPacket(packet.payload, packet.seq)
				} else {
					stm.resolveServerPacket(packet.payload, packet.seq)
				}
			}
		}
	}
}

func (stm *connStream) findStmtPacket (srv chan *packet, seq int) *packet {
	for {
		select {
		case packet, ok := <- stm.packets:
			if !ok {
				return nil
			}
			fmt.Println("packet.seq", packet.seq)
			if packet.seq == seq {
				return packet
			}
		case <-time.After(5 * time.Second):
			return nil
		}
	}
}

func (stm *connStream) resolveServerPacket(payload []byte, seq int) {

	var msg = ""
	if len(payload) == 0 {
		return
	}
	switch payload[0] {

	case 0xff:
		errorCode  := int(binary.LittleEndian.Uint16(payload[1:3]))
		errorMsg,_ := ReadStringFromByte(payload[4:])

		msg = GetNowStr(false)+"%s Err code:%s,Err msg:%s"
		msg  = fmt.Sprintf(msg, ErrorPacket, strconv.Itoa(errorCode), strings.TrimSpace(errorMsg))

	case 0x00:
		var pos = 1
		l,_,_ := LengthEncodedInt(payload[pos:])
		affectedRows := int(l)

		msg += GetNowStr(false)+"%s Effect Row:%s"
		msg = fmt.Sprintf(msg, OkPacket, strconv.Itoa(affectedRows))

	default:
		return
	}

	fmt.Println(msg)
}

func (stm *connStream) resolveClientPacket(payload []byte, seq int) {

	var msg string
	switch payload[0] {

	case COM_INIT_DB:

		msg = fmt.Sprintf("USE %s;\n", payload[1:])
	case COM_DROP_DB:

		msg = fmt.Sprintf("Drop DB %s;\n", payload[1:])
	case COM_CREATE_DB, COM_QUERY:
		statement := string(payload[1:])
		msg = fmt.Sprintf("%s %s\n", ComQueryRequestPacket, statement)
	case COM_STMT_PREPARE:
		serverPacket := stm.findStmtPacket(stm.packets, seq+1)
		if serverPacket == nil {
			log.Println("ERR : Not found stm packet")
			return
		}

		//fetch stm id
		stmtID := binary.LittleEndian.Uint32(serverPacket.payload[1:5])
		stmt := &Stmt{
			ID:    stmtID,
			Query: string(payload[1:]),
		}

		//record stm sql
		stm.stmtMap[stmtID] = stmt
		stmt.FieldCount = binary.LittleEndian.Uint16(serverPacket.payload[5:7])
		stmt.ParamCount = binary.LittleEndian.Uint16(serverPacket.payload[7:9])
		stmt.Args       = make([]interface{}, stmt.ParamCount)

		msg = PreparePacket+stmt.Query
	case COM_STMT_SEND_LONG_DATA:

		stmtID   := binary.LittleEndian.Uint32(payload[1:5])
		paramId  := binary.LittleEndian.Uint16(payload[5:7])
		stmt, _  := stm.stmtMap[stmtID]

		if stmt.Args[paramId] == nil {
			stmt.Args[paramId] = payload[7:]
		} else {
			if b, ok := stmt.Args[paramId].([]byte); ok {
				b = append(b, payload[7:]...)
				stmt.Args[paramId] = b
			}
		}
		return
	case COM_STMT_RESET:

		stmtID := binary.LittleEndian.Uint32(payload[1:5])
		stmt, _:= stm.stmtMap[stmtID]
		stmt.Args = make([]interface{}, stmt.ParamCount)
		return
	case COM_STMT_EXECUTE:
		var pos = 1
		stmtID := binary.LittleEndian.Uint32(payload[pos : pos+4])
		pos += 4
		var stmt *Stmt
		var ok bool
		if stmt, ok = stm.stmtMap[stmtID]; ok == false {
			log.Println("ERR : Not found stm id", stmtID)
			return
		}

		//params
		pos += 5
		if stmt.ParamCount > 0 {

			//（Null-Bitmap，len = (paramsCount + 7) / 8 byte）
			step := int((stmt.ParamCount + 7) / 8)
			nullBitmap := payload[pos : pos+step]
			pos += step

			//Parameter separator
			flag := payload[pos]

			pos++

			var pTypes  []byte
			var pValues []byte

			//if flag == 1
			//n （len = paramsCount * 2 byte）
			if flag == 1 {
				pTypes = payload[pos : pos+int(stmt.ParamCount)*2]
				pos += int(stmt.ParamCount) * 2
				pValues = payload[pos:]
			}

			//bind params
			err := stmt.BindArgs(nullBitmap, pTypes, pValues)
			if err != nil {
				log.Println("ERR : Could not bind params", err)
			}
		}
		msg = string(stmt.WriteToText())
	default:
		return
	}

	stm.outputer.Inputer() <- msg
}


func resolvePacketTo(r io.Reader, w io.Writer) (uint8, error) {
	header := make([]byte, 4)
	if n, err := io.ReadFull(r, header); err != nil {
		if n == 0 && err == io.EOF {
			return 0, io.EOF
		}
		return 0, errors.New("ERR : Unknown stream")
	}

	length := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)

	var seq uint8
	seq = header[3]

	if n, err := io.CopyN(w, r, int64(length)); err != nil {
		return 0, errors.New("ERR : Unknown stream")
	} else if n != int64(length) {
		return 0, errors.New("ERR : Unknown stream")
	} else {
		return seq, nil
	}

	return seq, nil
}

func ReadStringFromByte(b []byte) (string,int) {

	var l int
	l = bytes.IndexByte(b, 0x00)
	if l == -1 {
		l = len(b)
	}
	return string(b[0:l]), l
}

func GetNowStr(isClient bool) string {
	var msg string
	msg += time.Now().Format("2006-01-02 15:04:05")
	if isClient {
		msg += "| cli -> ser |"
	}else{
		msg += "| ser -> cli |"
	}
	return msg
}

func LengthEncodedInt(input []byte) (num uint64, isNull bool, n int) {

	switch input[0] {

	case 0xfb:
		n = 1
		isNull = true
		return
	case 0xfc:
		num = uint64(input[1]) | uint64(input[2])<<8
		n = 3
		return
	case 0xfd:
		num = uint64(input[1]) | uint64(input[2])<<8 | uint64(input[3])<<16
		n = 4
		return
	case 0xfe:
		num = uint64(input[1]) | uint64(input[2])<<8 | uint64(input[3])<<16 |
			uint64(input[4])<<24 | uint64(input[5])<<32 | uint64(input[6])<<40 |
			uint64(input[7])<<48 | uint64(input[8])<<56
		n = 9
		return
	}

	num = uint64(input[0])
	n = 1
	return
}

func LengthEncodedString(b []byte) ([]byte, bool, int, error) {

	num, isNull, n := LengthEncodedInt(b)
	if num < 1 {
		return nil, isNull, n, nil
	}

	n += int(num)

	if len(b) >= n {
		return b[n-int(num) : n], false, n, nil
	}
	return nil, false, n, io.EOF
}
