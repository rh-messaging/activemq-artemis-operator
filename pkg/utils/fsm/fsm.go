package fsm

type IState interface {
	Enter(stateFrom *IState)
	Update()
	Exit(stateTo *IState)
}

type State struct {
	name   string
	active bool
}

func MakeState(n string) State {
	return State{
		name: n,
	}
}

func NewState(n string) *State {
	s := MakeState(n)
	return &s
}

type IMachine interface {
	Add(s *IState)
	Remove(s *IState)
	Enter(stateFrom *IState)
	Update()
	Exit(stateTo *IState)
}

type Machine struct {
	currentStateIndex  int
	nextStateIndex     int
	previousStateIndex int
	numStates          int
	active             bool
	states             []*IState
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

func (m *Machine) Add(s *IState) {
	//if m.numStates >= len(m.states) {
	//newStatesLen := m.numStates+2
	//m.states = make([]*State, newStatesLen, 10)
	m.states = append(m.states, s)
	//}

	//m.states[m.numStates] = s
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

func (m *Machine) Enter(stateFrom *IState) {

	var s IState = *m.states[m.currentStateIndex]
	s.Enter(stateFrom)
}

func (m *Machine) Update() {

}

func (m *Machine) Exit(stateTo *IState) {
	var s IState = *m.states[m.currentStateIndex]
	s.Exit(stateTo)
}
