//go:generate mockgen -destination=replica_mock.go -package=plan . Replica
package plan

import (
	"errors"
	"sync"
)

type Replica interface {
	ExecuteAction(actionType, actionConfig string) (interface{}, error)
}

type ReplicaManager struct {
	sync.Mutex
	replicas []Replica
}

func (m *ReplicaManager) AddReplica(replica Replica) {
	m.Lock()
	m.replicas = append(m.replicas, replica)
	m.Unlock()
}

func (m *ReplicaManager) RemoveReplica(replica Replica) {
	m.Lock()
	defer m.Unlock()
	for i, r := range m.replicas {
		if r == replica {
			m.replicas = append(m.replicas[:i], m.replicas[i+1:]...)
			return
		}
	}
}

func (m *ReplicaManager) ExecuteAction(actionType, actionConfig string) (interface{}, error) {
	for _, replica := range m.replicas {
		result, err := replica.ExecuteAction(actionType, actionConfig)
		if err == nil {
			return result, nil
		}
	}
	return nil, errors.New("error running replicas")
}

var replicaManager = &ReplicaManager{
	replicas: []Replica{},
}

func GetReplicaManager() *ReplicaManager {
	return replicaManager
}
