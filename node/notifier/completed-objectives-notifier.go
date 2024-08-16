package notifier

import (
	"github.com/statechannels/go-nitro/internal/safesync"
	"github.com/statechannels/go-nitro/protocols"
)

type CompletedObjetivesNotifier struct {
	completedObjectivesListeners *safesync.Map[*completedObjectivesListeners]
}

func NewCompletedObjectivesNotifier() *CompletedObjetivesNotifier {
	return &CompletedObjetivesNotifier{
		completedObjectivesListeners: &safesync.Map[*completedObjectivesListeners]{},
	}
}

// RegisterForAllCompletedObjectives returns a buffered channel that will receive all completed objective IDs
func (con *CompletedObjetivesNotifier) RegisterForAllCompletedObjectives() <-chan protocols.ObjectiveId {
	li, _ := con.completedObjectivesListeners.LoadOrStore(ALL_NOTIFICATIONS, newCompletedObjectivesListeners())
	return li.createNewListener()
}

// BroadcastCompletedObjective broadcasts the completed objectives to all the listeners
func (con *CompletedObjetivesNotifier) BroadcastCompletedObjective(objectiveId protocols.ObjectiveId) {
	li, _ := con.completedObjectivesListeners.LoadOrStore(ALL_NOTIFICATIONS, newCompletedObjectivesListeners())
	li.broadcastCompletedObjective(objectiveId)
}

// Close closes the notifier and all listeners
func (con *CompletedObjetivesNotifier) Close() error {
	var err error
	con.completedObjectivesListeners.Range(func(k string, v *completedObjectivesListeners) bool {
		err = v.Close()
		return err == nil
	})

	return err
}
