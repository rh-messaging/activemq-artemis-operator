package fsm

type IState interface {
	ID() int
	Enter(previousStateID int) error
	Update() (error, int)
	Exit() error
}

type State struct {
	name   string
	id     int
	active bool
}

func MakeState(n string, i int) State {
	return State{
		name:   n,
		id:     i,
		active: false,
	}
}

func NewState(n string, i int) *State {
	s := MakeState(n, i)
	return &s
}

type IMachine interface {
	Add(s *IState)
	Remove(s *IState)
	//	ID() int
	Enter(previousStateID int) error
	Update() (error, int)
	Transition() error
	Exit() error
	SetCurrentState(state IState)
}

type Machine struct {
	//id					int
	currentStateID  int
	nextStateID     int
	previousStateID int
	numStates       int
	active          bool
	states          []*IState
	currentState    IState
}

func CreateMachine(currentID int, nextID int, prevID int, numStats int, isActive bool) *Machine {
	m := Machine{
		currentStateID:  currentID,
		nextStateID:     nextID,
		previousStateID: prevID,
		numStates:       numStats,
		active:          isActive,
		states:          make([]*IState, 0, 10),
	}
	return &m
}

func MakeMachine() Machine {
	return Machine{
		states: make([]*IState, 0, 10),
	}
}

func NewMachine() *Machine {
	m := MakeMachine()
	return &m
}

func (m *Machine) SetCurrentState(state IState) {
	m.currentState = state
}

func (m *Machine) GetCurrentStateID() int {
	return m.currentStateID
}

func (m *Machine) GetNextStateID() int {
	return m.nextStateID
}

func (m *Machine) GetPreviousStateID() int {
	return m.previousStateID
}

func (m *Machine) GetNumStates() int {
	return m.numStates
}

func (m *Machine) GetActive() bool {
	return m.active
}

func (m *Machine) GetState(stateID int) *IState {
	for _, state := range m.states {
		if (*state).ID() == stateID {
			return state
		}
	}
	return nil
}

func (m *Machine) Add(s *IState) {
	m.states = append(m.states, s)
	m.numStates++
}

func (m *Machine) Remove(s *IState) {
	currentStateIndex := 0
	for nextState := m.states[currentStateIndex]; nil != nextState; currentStateIndex++ {
		if nextState == s {
			nextState = nil
		}
	}
}

//func (m *Machine) ID() int {
//	return -1 // i.e. not set
//}

func (m *Machine) Enter(startStateID int) error {
	m.previousStateID = -1
	m.currentStateID = startStateID
	m.nextStateID = m.currentStateID
	m.currentState = *m.states[m.currentStateID]
	err := m.currentState.Enter(m.previousStateID)
	return err
}

func (m *Machine) GetIDCurrentState() int {
	if m.currentState != nil {
		return m.currentState.ID()
	}
	return -1
}

func (m *Machine) Update() (error, int) {
	m.currentStateID = m.currentState.ID()
	var err error
	err, m.nextStateID = m.currentState.Update()
	if m.nextStateID != m.currentStateID {
		m.Transition()
	}
	return err, m.nextStateID
}

func (m *Machine) Transition() error {
	m.currentState.Exit()
	m.previousStateID = m.currentStateID
	m.currentState = *m.states[m.nextStateID]
	err := m.currentState.Enter(m.previousStateID)
	return err
}

func (m *Machine) Exit() error {
	err := m.currentState.Exit()
	return err
}
