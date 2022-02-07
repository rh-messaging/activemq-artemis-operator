/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package common

import "sync"

type StateManager struct {
	*sync.Mutex
	state map[string]interface{}
}

var singleton *StateManager
var once sync.Once

func GetStateManager() *StateManager {
	once.Do(func() {
		singleton = &StateManager{Mutex: &sync.Mutex{}}
		singleton.state = make(map[string]interface{})
	})
	return singleton
}

func (sm *StateManager) GetState(key string) interface{} {
	sm.Lock()
	defer sm.Unlock()
	return sm.state[key]
}

func (sm *StateManager) SetState(key string, value interface{}) {
	sm.Lock()
	defer sm.Unlock()
	sm.state[key] = value
}

func (sm *StateManager) Clear() {
	sm.Lock()
	defer sm.Unlock()
	sm.state = make(map[string]interface{})
}
