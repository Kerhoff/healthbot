package bot

import (
	"sync"
	"time"
)

type StateKey string

const (
	StateIdle StateKey = ""

	// Fasting
	StateFastingNotes StateKey = "fasting_notes"

	// Weight
	StateWeightInput StateKey = "weight_input"

	// Body measurements wizard
	StateMeasureChest  StateKey = "measure_chest"
	StateMeasureWaist  StateKey = "measure_waist"
	StateMeasureHips   StateKey = "measure_hips"
	StateMeasureBicep  StateKey = "measure_bicep"
	StateMeasureThigh  StateKey = "measure_thigh"
	StateMeasureDone   StateKey = "measure_done"

	// Nutrition manual
	StateNutritionMealType StateKey = "nutrition_meal_type"
	StateNutritionCalories StateKey = "nutrition_calories"
	StateNutritionProtein  StateKey = "nutrition_protein"
	StateNutritionCarbs    StateKey = "nutrition_carbs"
	StateNutritionFat      StateKey = "nutrition_fat"

	// Nutrition photo
	StateNutritionPhotoMealType StateKey = "nutrition_photo_meal_type"
	StateNutritionPhotoWait     StateKey = "nutrition_photo_wait"

	// Medication add wizard
	StateMedAddName      StateKey = "med_add_name"
	StateMedAddDosage    StateKey = "med_add_dosage"
	StateMedAddUnit      StateKey = "med_add_unit"
	StateMedAddFrequency StateKey = "med_add_frequency"
	StateMedAddTimes     StateKey = "med_add_times"
)

type UserState struct {
	State   StateKey
	Data    map[string]any
	Updated time.Time
}

type FSM struct {
	mu     sync.Map
	ttl    time.Duration
}

func NewFSM(ttl time.Duration) *FSM {
	f := &FSM{ttl: ttl}
	go f.gc()
	return f
}

func (f *FSM) Get(userID int64) *UserState {
	v, ok := f.mu.Load(userID)
	if !ok {
		return &UserState{State: StateIdle, Data: map[string]any{}}
	}
	return v.(*UserState)
}

func (f *FSM) Set(userID int64, state StateKey, data map[string]any) {
	f.mu.Store(userID, &UserState{
		State:   state,
		Data:    data,
		Updated: time.Now(),
	})
}

func (f *FSM) Clear(userID int64) {
	f.mu.Delete(userID)
}

func (f *FSM) gc() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		now := time.Now()
		f.mu.Range(func(k, v any) bool {
			if now.Sub(v.(*UserState).Updated) > f.ttl {
				f.mu.Delete(k)
			}
			return true
		})
	}
}
