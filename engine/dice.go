package engine

import (
	"log"
	"math/rand"
	"time"
)

// DiceRoller interface for injectable randomness
type DiceRoller interface {
	// Roll returns a random number between 1 and sides (inclusive)
	Roll(sides int) int
}

// RandomDice uses real randomness
type RandomDice struct {
	rng *rand.Rand
}

// NewRandomDice creates a new random dice roller
func NewRandomDice() *RandomDice {
	seed := time.Now().UnixNano()
	log.Printf("Dice seed: %d", seed)
	return &RandomDice{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// NewSeededDice creates a dice roller with a specific seed (for testing)
func NewSeededDice(seed int64) *RandomDice {
	return &RandomDice{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// Roll returns a random number between 1 and sides (inclusive)
func (d *RandomDice) Roll(sides int) int {
	return d.rng.Intn(sides) + 1
}

// FixedDice always returns predetermined values (for testing)
type FixedDice struct {
	values []int
	index  int
}

// NewFixedDice creates a dice that returns predetermined values in sequence
func NewFixedDice(values ...int) *FixedDice {
	return &FixedDice{
		values: values,
		index:  0,
	}
}

// Roll returns the next predetermined value
func (d *FixedDice) Roll(sides int) int {
	if len(d.values) == 0 {
		return 1
	}
	value := d.values[d.index%len(d.values)]
	d.index++
	// Clamp to valid range
	if value < 1 {
		value = 1
	}
	if value > sides {
		value = sides
	}
	return value
}

// Reset resets the fixed dice to start from the beginning
func (d *FixedDice) Reset() {
	d.index = 0
}
