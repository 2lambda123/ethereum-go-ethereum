// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package nodestate

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"
)

func testSetup(flagPersist []bool, fieldType []reflect.Type) Setup {
	flags := make([]*flagDefinition, len(flagPersist))
	for i, persist := range flagPersist {
		if persist {
			flags[i] = NewPersistentFlag(fmt.Sprintf("flag-%d", i))
		} else {
			flags[i] = NewFlag(fmt.Sprintf("flag-%d", i))
		}
	}
	fields := make([]*fieldDefinition, len(fieldType))
	for i, ftype := range fieldType {
		switch ftype {
		case reflect.TypeOf(uint64(0)):
			fields[i] = NewPersistentField(fmt.Sprintf("field-%d", i), ftype, uint64FieldEnc, uint64FieldDec)
		case reflect.TypeOf(""):
			fields[i] = NewPersistentField(fmt.Sprintf("field-%d", i), ftype, stringFieldEnc, stringFieldDec)
		default:
			fields[i] = NewField(fmt.Sprintf("field-%d", i), ftype)
		}
	}
	return Setup{flags, fields}
}

func regSetup(ns *NodeStateMachine, setup Setup) ([]bitMask, []int) {
	masks := make([]bitMask, len(setup.Flags))
	for i, flag := range setup.Flags {
		masks[i] = ns.StateMask(flag)
	}
	fieldIndex := make([]int, len(setup.Fields))
	for i, field := range setup.Fields {
		fieldIndex[i] = ns.FieldIndex(field)
	}
	return masks, fieldIndex
}

func testNode(b byte) *enode.Node {
	r := &enr.Record{}
	r.SetSig(dummyIdentity{b}, []byte{42})
	n, _ := enode.New(dummyIdentity{b}, r)
	return n
}

func TestCallback(t *testing.T) {
	mdb, clock := rawdb.NewMemoryDatabase(), &mclock.Simulated{}

	s := testSetup([]bool{false, false, false}, nil)
	ns := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	flags, _ := regSetup(ns, s)

	set0 := make(chan struct{}, 1)
	set1 := make(chan struct{}, 1)
	set2 := make(chan struct{}, 1)
	ns.SubscribeState(flags[0], func(n *enode.Node, oldState, newState bitMask) { set0 <- struct{}{} })
	ns.SubscribeState(flags[1], func(n *enode.Node, oldState, newState bitMask) { set1 <- struct{}{} })
	ns.SubscribeState(flags[2], func(n *enode.Node, oldState, newState bitMask) { set2 <- struct{}{} })

	ns.Start()

	ns.SetState(testNode(1), flags[0], 0, 0)
	ns.SetState(testNode(1), flags[1], 0, time.Second)
	ns.SetState(testNode(1), flags[2], 0, 2*time.Second)

	for i := 0; i < 3; i++ {
		select {
		case <-set0:
		case <-set1:
		case <-set2:
		case <-time.After(time.Second):
			t.Fatalf("failed to invoke callback")
		}
	}
}

func TestPersistentFlags(t *testing.T) {
	mdb, clock := rawdb.NewMemoryDatabase(), &mclock.Simulated{}

	s := testSetup([]bool{true, true, true, false}, nil)
	ns := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	flags, _ := regSetup(ns, s)

	saveNode := make(chan *nodeInfo, 5)
	ns.saveNodeHook = func(node *nodeInfo) {
		saveNode <- node
	}

	ns.Start()

	ns.SetState(testNode(1), flags[0], 0, time.Second) // state with timeout should not be saved
	ns.SetState(testNode(2), flags[1], 0, 0)
	ns.SetState(testNode(3), flags[2], 0, 0)
	ns.SetState(testNode(4), flags[3], 0, 0)
	ns.SetState(testNode(5), flags[0], 0, 0)
	ns.Persist(testNode(5))
	select {
	case <-saveNode:
	case <-time.After(time.Second):
		t.Fatalf("Timeout")
	}
	ns.Stop()

	for i := 0; i < 2; i++ {
		select {
		case <-saveNode:
		case <-time.After(time.Second):
			t.Fatalf("Timeout")
		}
	}
	select {
	case <-saveNode:
		t.Fatalf("Unexpected saveNode")
	case <-time.After(time.Millisecond * 100):
	}
}

func TestSetField(t *testing.T) {
	mdb, clock := rawdb.NewMemoryDatabase(), &mclock.Simulated{}

	s := testSetup([]bool{true}, []reflect.Type{reflect.TypeOf("")})
	ns := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	flags, fields := regSetup(ns, s)

	saveNode := make(chan *nodeInfo, 1)
	ns.saveNodeHook = func(node *nodeInfo) {
		saveNode <- node
	}

	ns.Start()

	// Set field before setting state
	ns.SetField(testNode(1), fields[0], "hello world")
	field := ns.GetField(testNode(1), fields[0])
	if field != nil {
		t.Fatalf("Field shouldn't be set before setting states")
	}
	// Set field after setting state
	ns.SetState(testNode(1), flags[0], 0, 0)
	ns.SetField(testNode(1), fields[0], "hello world")
	field = ns.GetField(testNode(1), fields[0])
	if field == nil {
		t.Fatalf("Field should be set after setting states")
	}
	if err := ns.SetField(testNode(1), fields[0], 123); err != errInvalidField {
		t.Fatalf("Invalid field should be rejected")
	}
	// Dirty node should be written back
	ns.Stop()
	select {
	case <-saveNode:
	case <-time.After(time.Second):
		t.Fatalf("Timeout")
	}
}

func TestUnsetField(t *testing.T) {
	mdb, clock := rawdb.NewMemoryDatabase(), &mclock.Simulated{}

	s := testSetup([]bool{false}, []reflect.Type{reflect.TypeOf("")})
	ns := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	flags, fields := regSetup(ns, s)

	ns.Start()

	ns.SetState(testNode(1), flags[0], 0, time.Second)
	ns.SetField(testNode(1), fields[0], "hello world")

	ns.SetState(testNode(1), 0, flags[0], 0)
	if field := ns.GetField(testNode(1), fields[0]); field != nil {
		t.Fatalf("Field should be unset")
	}
}

func TestSetState(t *testing.T) {
	mdb, clock := rawdb.NewMemoryDatabase(), &mclock.Simulated{}

	s := testSetup([]bool{false, false, false}, nil)
	ns := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	flags, _ := regSetup(ns, s)

	type change struct{ old, new bitMask }
	set := make(chan change, 1)
	ns.SubscribeState(flags[0]|flags[1], func(n *enode.Node, oldState, newState bitMask) {
		set <- change{
			old: oldState,
			new: newState,
		}
	})

	ns.Start()

	check := func(expectOld, expectNew bitMask, expectChange bool) {
		if expectChange {
			select {
			case c := <-set:
				if c.old != expectOld {
					t.Fatalf("Old state mismatch")
				}
				if c.new != expectNew {
					t.Fatalf("New state mismatch")
				}
			case <-time.After(time.Second):
			}
			return
		}
		select {
		case <-set:
			t.Fatalf("Unexpected change")
		case <-time.After(time.Millisecond * 100):
			return
		}
	}
	ns.SetState(testNode(1), flags[0], 0, 0)
	check(0, flags[0], true)

	ns.SetState(testNode(1), flags[1], 0, 0)
	check(flags[0], flags[0]|flags[1], true)

	ns.SetState(testNode(1), flags[2], 0, 0)
	check(0, 0, false)

	ns.SetState(testNode(1), 0, flags[0], 0)
	check(flags[0]|flags[1], flags[1], true)

	ns.SetState(testNode(1), 0, flags[1], 0)
	check(flags[1], 0, true)

	ns.SetState(testNode(1), 0, flags[2], 0)
	check(0, 0, false)

	ns.SetState(testNode(1), flags[0]|flags[1], 0, time.Second)
	check(0, flags[0]|flags[1], true)
	clock.Run(time.Second)
	check(flags[0]|flags[1], 0, true)
}

func uint64FieldEnc(field interface{}) ([]byte, error) {
	if u, ok := field.(uint64); ok {
		enc, err := rlp.EncodeToBytes(&u)
		return enc, err
	} else {
		return nil, errInvalidField
	}
}

func uint64FieldDec(enc []byte) (interface{}, error) {
	var u uint64
	err := rlp.DecodeBytes(enc, &u)
	return u, err
}

func stringFieldEnc(field interface{}) ([]byte, error) {
	if s, ok := field.(string); ok {
		return []byte(s), nil
	} else {
		return nil, errInvalidField
	}
}

func stringFieldDec(enc []byte) (interface{}, error) {
	return string(enc), nil
}

func TestPersistentFields(t *testing.T) {
	mdb, clock := rawdb.NewMemoryDatabase(), &mclock.Simulated{}

	s := testSetup([]bool{true}, []reflect.Type{reflect.TypeOf(uint64(0)), reflect.TypeOf("")})
	ns := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	flags, fields := regSetup(ns, s)

	ns.Start()
	ns.SetState(testNode(1), flags[0], 0, 0)
	ns.SetField(testNode(1), fields[0], uint64(100))
	ns.SetField(testNode(1), fields[1], "hello world")
	ns.Stop()

	ns2 := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)

	ns2.Start()
	field0 := ns2.GetField(testNode(1), fields[0])
	if !reflect.DeepEqual(field0, uint64(100)) {
		t.Fatalf("Field changed")
	}
	field1 := ns2.GetField(testNode(1), fields[1])
	if !reflect.DeepEqual(field1, "hello world") {
		t.Fatalf("Field changed")
	}

	// additional registration
	s = testSetup([]bool{true, true}, []reflect.Type{reflect.TypeOf(uint64(0)), reflect.TypeOf(""), reflect.TypeOf(uint32(0))})
	// Different order
	s.Flags[0], s.Flags[1] = s.Flags[1], s.Flags[0]
	s.Fields[0], s.Fields[1] = s.Fields[1], s.Fields[0]
	ns3 := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	_, fields = regSetup(ns3, s)

	ns3.Start()
	field0 = ns3.GetField(testNode(1), fields[1])
	if !reflect.DeepEqual(field0, uint64(100)) {
		t.Fatalf("Field changed")
	}
	field1 = ns3.GetField(testNode(1), fields[0])
	if !reflect.DeepEqual(field1, "hello world") {
		t.Fatalf("Field changed")
	}
}

func TestFieldSub(t *testing.T) {
	mdb, clock := rawdb.NewMemoryDatabase(), &mclock.Simulated{}

	s := testSetup([]bool{true}, []reflect.Type{reflect.TypeOf(uint64(0))})
	ns := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	flags, fields := regSetup(ns, s)

	var (
		lastState                  bitMask
		lastOldValue, lastNewValue interface{}
	)
	ns.SubscribeField(fields[0], func(n *enode.Node, state bitMask, oldValue, newValue interface{}) {
		lastState, lastOldValue, lastNewValue = state, oldValue, newValue
	})
	check := func(state bitMask, oldValue, newValue interface{}) {
		if lastState != state || lastOldValue != oldValue || lastNewValue != newValue {
			t.Fatalf("Incorrect field sub callback (expected [%v %v %v], got [%v %v %v])", state, oldValue, newValue, lastState, lastOldValue, lastNewValue)
		}
	}
	ns.Start()
	ns.SetState(testNode(1), flags[0], 0, 0)
	ns.SetField(testNode(1), fields[0], uint64(100))
	check(flags[0], nil, uint64(100))
	ns.Stop()
	check(offlineState, uint64(100), nil)

	ns2 := NewNodeStateMachine(mdb, []byte("-ns"), clock, s)
	ns2.SubscribeField(fields[0], func(n *enode.Node, state bitMask, oldValue, newValue interface{}) {
		lastState, lastOldValue, lastNewValue = state, oldValue, newValue
	})
	ns2.Start()
	check(offlineState, nil, uint64(100))
	ns2.SetState(testNode(1), 0, flags[0], 0)
	check(0, uint64(100), nil)
	ns2.Stop()
}