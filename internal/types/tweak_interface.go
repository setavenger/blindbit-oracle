package types

type TweakDusted interface {
	HighestValue() uint64
	Tweak() [33]byte // todo pointer to show none?
}
