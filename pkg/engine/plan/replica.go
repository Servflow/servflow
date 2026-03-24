//go:generate mockgen -destination=replica_mock.go -package=plan . Replica
package plan

import "errors"

type Replica interface {
	ExecuteAction(actionType, actionConfig string, contextVariables map[string]interface{}) (interface{}, error)
}

type ReplicaManager struct {
	replicas []Replica
}

func (m *ReplicaManager) AddReplica(replica Replica) {
	m.replicas = append(m.replicas, replica)
}

func (m *ReplicaManager) ExecuteAction(actionType, actionConfig string, contextVariables map[string]interface{}) (interface{}, error) {
	for _, replica := range m.replicas {
		result, err := replica.ExecuteAction(actionType, actionConfig, contextVariables)
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
