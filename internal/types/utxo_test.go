package types

import "testing"

func TestFindBiggestRemainingUTXO(t *testing.T) {

	utxo := UTXO{Value: 1500}
	utxos := []UTXO{
		{Value: 1000},
		{Value: 2000},
		{Value: 1000},
	}

	nextBiggest, err := FindBiggestRemainingUTXO(utxo, utxos)
	if err != nil {
		t.Errorf("FindBiggestRemainingUTXO returned an error %s", err)
		return
	}
	if nextBiggest != nil {
		t.Errorf("FindBiggestRemainingUTXO returned a non-nil value %v", nextBiggest)
	}

	utxo = UTXO{Value: 5000}
	utxos = []UTXO{
		{Value: 1000},
		{Value: 2000},
		{Value: 3000},
	}

	nextBiggest, err = FindBiggestRemainingUTXO(utxo, utxos)
	if err != nil {
		t.Errorf("FindBiggestRemainingUTXO returned an error %s", err)
		return
	}
	if *nextBiggest != 3000 {
		t.Errorf("FindBiggestRemainingUTXO returned a non-3000 value %v", nextBiggest)
	}

	utxo = UTXO{Value: 5000}
	utxos = []UTXO{
		{Value: 1000},
		{Value: 2000},
		{Value: 5001},
		{Value: 7000, Spent: true},
	}

	nextBiggest, err = FindBiggestRemainingUTXO(utxo, utxos)
	if err != nil {
		t.Errorf("FindBiggestRemainingUTXO returned an error %s", err)
		return
	}
	if nextBiggest != nil {
		t.Errorf("FindBiggestRemainingUTXO returned a non-nil value %v", nextBiggest)
	}

	utxo = UTXO{Value: 5000}
	utxos = []UTXO{
		{Value: 1000},
		{Value: 2000},
		{Value: 4999},
		{Value: 7000, Spent: true},
	}

	nextBiggest, err = FindBiggestRemainingUTXO(utxo, utxos)
	if err != nil {
		t.Errorf("FindBiggestRemainingUTXO returned an error %s", err)
		return
	}
	if *nextBiggest != 4999 {
		t.Errorf("FindBiggestRemainingUTXO returned a non-4999 value %v", nextBiggest)
	}

	utxo = UTXO{Value: 5000, Spent: true}
	utxos = []UTXO{
		{Value: 5000, Spent: true},
	}

	nextBiggest, err = FindBiggestRemainingUTXO(utxo, utxos)
	if err != nil {
		t.Errorf("FindBiggestRemainingUTXO returned an error %s", err)
		return
	}
	if nextBiggest != nil {
		t.Errorf("FindBiggestRemainingUTXO returned a non-nil value %v", nextBiggest)
	}
}
