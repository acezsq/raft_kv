package raft

import (
	"sort"
	"time"
)

type LogEntry struct {
	Term         int
	Command      interface{}
	CommandValid bool
}

type AppendEntriesArgs struct {
	Term     int
	LeaderId int

	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry

	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool

	ConflictIndex int
	ConflictTerm  int
}

// 对来自Leader的心跳/日志同步请求进行回调

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.Success = false
	// 如果Leader请求的任期小于当前任期
	if args.Term < rf.currentTerm {
		LOG(rf.me, rf.currentTerm, DLog2, "<- S%d, Reject log", args.LeaderId)
		return
	}

	defer rf.resetElectionTimerLocked()

	// 如果Leader请求的任期比当前任期大
	// ？？？这里当我是candidate且和leader的任期相同时，不用将votedFor置为-1吗？ 一个任期只能投票一次
	if args.Term >= rf.currentTerm {
		rf.becomeFollowerLocked(args.Term)
	}

	if args.PrevLogIndex >= len(rf.log) {
		reply.ConflictIndex = len(rf.log)
		reply.ConflictTerm = InvalidTerm
		return
	}

	if rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.ConflictTerm = rf.log[args.PrevLogIndex].Term
		reply.ConflictIndex = rf.firstLogFor(reply.ConflictTerm)
		return
	}

	rf.log = append(rf.log[:args.PrevLogIndex+1], args.Entries...)
	rf.persistLocked()
	reply.Success = true

	//
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = args.LeaderCommit
		rf.applyCond.Signal()
	}
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) getMajorityIndexLocked() int {
	tmpIndexes := make([]int, len(rf.peers))
	copy(tmpIndexes, rf.matchIndex)
	sort.Ints(tmpIndexes)
	majorityIdx := (len(rf.peers) - 1) / 2
	return tmpIndexes[majorityIdx]
}

func (rf *Raft) startReplication(term int) bool {
	replicationToPeer := func(peer int, args *AppendEntriesArgs) {
		reply := &AppendEntriesReply{}
		ok := rf.sendAppendEntries(peer, args, reply)

		rf.mu.Lock()
		defer rf.mu.Unlock()
		if !ok {
			LOG(rf.me, rf.currentTerm, DLeader, "Leader[T%d] -> %s[T%d]", term, rf.role, rf.currentTerm)
			return
		}
		if reply.Term > rf.currentTerm {
			rf.becomeFollowerLocked(reply.Term)
			return
		}

		if rf.contextLostLocked(Leader, term) {
			return
		}

		// 如果返回失败，那么需要往前试探
		if !reply.Success {
			preIndex := rf.nextIndex[peer]
			if reply.ConflictTerm == InvalidTerm {
				rf.nextIndex[peer] = reply.ConflictIndex
			} else {
				firstIndex := rf.firstLogFor(reply.ConflictTerm)
				if firstIndex != InvalidIndex {
					rf.nextIndex[peer] = firstIndex
				} else {
					rf.nextIndex[peer] = reply.ConflictIndex
				}
			}
			if preIndex < rf.nextIndex[peer] {
				rf.nextIndex[peer] = preIndex
			}
			return
		}

		rf.matchIndex[peer] = args.PrevLogIndex + len(args.Entries)
		rf.nextIndex[peer] = rf.matchIndex[peer] + 1

		//
		majorityMatched := rf.getMajorityIndexLocked()
		if majorityMatched > rf.commitIndex && rf.log[majorityMatched].Term == rf.currentTerm {
			rf.commitIndex = majorityMatched
			rf.applyCond.Signal()
		}
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.contextLostLocked(Leader, term) {
		LOG(rf.me, rf.currentTerm, DLeader, "Leader[T%d] -> %s[T%d]", term, rf.role, rf.currentTerm)
		return false
	}

	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			// 更新leader的matchIndex
			rf.matchIndex[i] = len(rf.log) - 1
			rf.nextIndex[i] = len(rf.log)
			continue
		}

		prevIdx := rf.nextIndex[i] - 1
		prevTerm := rf.log[prevIdx].Term
		args := &AppendEntriesArgs{
			Term:         rf.currentTerm,
			LeaderId:     rf.me,
			PrevLogIndex: prevIdx,
			PrevLogTerm:  prevTerm,
			Entries:      rf.log[prevIdx+1:],
			//Entries:      append([]LogEntry(nil), rf.log[prevIdx+1:]...),
			LeaderCommit: rf.commitIndex,
		}
		go replicationToPeer(i, args)

	}
	return true
}

// Leader发送心跳

func (rf *Raft) replicationTicker(term int) {
	for !rf.killed() {
		ok := rf.startReplication(term)
		if !ok {
			break
		}
		time.Sleep(replicateInterval)
	}
}
