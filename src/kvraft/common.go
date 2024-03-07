package kvraft

import (
	"fmt"
	"time"
)

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	ErrTimeout     = "ErrTimeout"
)

const ClientRequestTimeout = 500 * time.Millisecond

type OperationType uint8

const (
	OpGet OperationType = iota
	OpPut
	OpAppend
)

type Err string

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Key    string
	Value  string
	OpType OperationType

	ClientId int64
	SeqId    int64
}

type OpReply struct {
	Value string
	Err   Err
}

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	ClientId int64
	SeqId    int64
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
}

type GetReply struct {
	Err   Err
	Value string
}

func getOperationType(v string) OperationType {
	switch v {
	case "Put":
		return OpPut
	case "Append":
		return OpAppend
	default:
		panic(fmt.Sprintf("unknow operation type %s", v))
	}
}

type LastOperationInfo struct {
	SeqId int64
	Reply *OpReply
}
