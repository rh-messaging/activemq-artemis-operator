package controllers

import "testing"

func TestHexShaHashOfMap(t *testing.T) {

	nilOne := HexShaHashOfMap(nil)
	nilTwo := HexShaHashOfMap(nil)

	if nilOne != nilTwo {
		t.Errorf("HexShaHashOfMap(nil) = %v, want %v", nilOne, nilTwo)
	}

	props := map[string]string{"a": "a", "b": "b"}

	propsOriginal := HexShaHashOfMap(props)

	// modify
	props["c"] = "c"

	propsModified := HexShaHashOfMap(props)

	if propsOriginal == propsModified {
		t.Errorf("HexShaHashOfMap(props mod) = %v, want %v", propsOriginal, propsModified)
	}

	// revert
	delete(props, "c")

	if propsOriginal != HexShaHashOfMap(props) {
		t.Errorf("HexShaHashOfMap(props) with revert = %v, want %v", propsOriginal, HexShaHashOfMap(props))
	}

	// modify further
	delete(props, "a")

	if propsOriginal == HexShaHashOfMap(props) {
		t.Errorf("HexShaHashOfMap(props) with just a = %v, want %v", propsOriginal, HexShaHashOfMap(props))
	}

}
