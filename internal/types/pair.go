package types

type Pair interface {
	SerialiseKey() ([]byte, error) // in case it fails we can abort
	SerialiseData() ([]byte, error)
	DeSerialiseKey([]byte) error  // needs to be implemented with pointer method in order to insert data into struct
	DeSerialiseData([]byte) error // needs to be implemented with pointer method in order to insert data into struct
}

type PairFactory func() Pair
