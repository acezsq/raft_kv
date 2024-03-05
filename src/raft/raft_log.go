package raft

import (
	"course/labgob"
	"fmt"
)

type RaftLog struct {
	snapLastIdx  int
	snapLastTerm int

	// 截断的时候index及之前的log
	snapshot []byte

	// 截断处之后的log
	// 下标为0的位置留空，Term字段设置为snapLastTerm
	// 下标为1处放snapLastIdx+1的log
	tailLog []LogEntry
}

func (rl *RaftLog) installSnapshot(index, term int, snapshot []byte) {
	rl.snapLastIdx = index
	rl.snapLastTerm = term
	rl.snapshot = snapshot

	newLog := make([]LogEntry, 0, 1)
	newLog = append(newLog, LogEntry{
		Term: rl.snapLastTerm,
	})
	rl.tailLog = newLog
}

func (rl *RaftLog) doSnapshot(index int, snapshot []byte) {
	idx := rl.idx(index)

	rl.snapLastIdx = index
	rl.snapLastTerm = rl.tailLog[idx].Term
	rl.snapshot = snapshot

	newLog := make([]LogEntry, 0, rl.size()-rl.snapLastIdx)
	newLog = append(newLog, LogEntry{
		Term: rl.snapLastTerm,
	})
	newLog = append(newLog, rl.tailLog[idx+1:]...)
	rl.tailLog = newLog
}

// RaftLog的构造函数
func NewLog(snapLastIdx, snapLastTerm int, snapshot []byte, entries []LogEntry) *RaftLog {
	rl := &RaftLog{
		snapLastIdx:  snapLastIdx,
		snapLastTerm: snapLastTerm,
		snapshot:     snapshot,
	}
	rl.tailLog = make([]LogEntry, 0, 1+len(entries))
	rl.tailLog = append(rl.tailLog, LogEntry{Term: snapLastTerm})
	rl.tailLog = append(rl.tailLog, entries...)
	return rl
}

// 从持久化存储中读取数据
func (rl *RaftLog) readPersist(d *labgob.LabDecoder) error {
	var lastIdx int
	if err := d.Decode(&lastIdx); err != nil {
		return fmt.Errorf("decode last include index fail")
	}
	rl.snapLastIdx = lastIdx

	var lastTerm int
	if err := d.Decode(&lastTerm); err != nil {
		return fmt.Errorf("decode last include term fail")
	}
	rl.snapLastTerm = lastTerm

	var log []LogEntry
	if err := d.Decode(&log); err != nil {
		return fmt.Errorf("decode tail log fail")
	}
	rl.tailLog = log
	return nil
}

// 持久化数据
func (rl *RaftLog) persist(e *labgob.LabEncoder) {
	e.Encode(rl.snapLastIdx)
	e.Encode(rl.snapLastTerm)
	e.Encode(rl.tailLog)
}

// RaftLog的长度，注意这里算上了tailLog的0处
func (rl *RaftLog) size() int {
	return rl.snapLastIdx + len(rl.tailLog)
}

// 根据原index获取该index在tailLog的index
func (rl *RaftLog) idx(logicIdx int) int {
	if logicIdx < rl.snapLastIdx || logicIdx >= rl.size() {
		panic(fmt.Sprintf("%d is out of [%d %d]", logicIdx, rl.snapLastIdx+1, rl.size()-1))
	}
	return logicIdx - rl.snapLastIdx
}

// 获取logicIdx处的logEntry
func (rl *RaftLog) at(logicIdx int) LogEntry {
	return rl.tailLog[rl.idx(logicIdx)]
}

// 获取RaftLog最后一条日志的索引和任期
func (rl *RaftLog) last() (idx, term int) {
	return rl.size() - 1, rl.tailLog[len(rl.tailLog)-1].Term
}

// 获取tailLog从startIdx到末尾的日志,startIdx是logic的
func (rl *RaftLog) tail(startIdx int) []LogEntry {
	if startIdx >= rl.size() {
		return nil
	}
	return rl.tailLog[rl.idx(startIdx):]
}

// 获取tailLog中第一个任期为term的logicIdx
func (rl *RaftLog) firstFor(term int) int {
	for idx, entry := range rl.tailLog {
		if entry.Term == term {
			return idx + rl.snapLastIdx
		} else if entry.Term > term {
			break
		}
	}
	return InvalidIndex
}

// 向tailLog中增加logEntry
func (rl *RaftLog) append(e LogEntry) {
	rl.tailLog = append(rl.tailLog, e)
}

// 设置tailLog为从logic的prevIdx处向前截取 + entries
func (rl *RaftLog) appendFrom(prevIdx int, entries []LogEntry) {
	rl.tailLog = append(rl.tailLog[:rl.idx(prevIdx)+1], entries...)
}
