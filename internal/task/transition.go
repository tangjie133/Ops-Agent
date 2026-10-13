package task

import "fmt"

// CanTransition 判断状态流转是否合法。
func CanTransition(from, to Status) bool {
	if from == to {
		return false
	}
	if isTerminal(from) {
		return false
	}
	if from == StatusFailed || from == StatusNeedsManualIntervention {
		if to == StatusCollectingContext {
			return true
		}
	}
	if isEscape(to) {
		return true
	}
	next, ok := forward[from]
	if !ok {
		return false
	}
	for _, n := range next {
		if n == to {
			return true
		}
	}
	return false
}

func isTerminal(s Status) bool {
	switch s {
	case StatusDone, StatusCancelled, StatusRolledBack:
		return true
	default:
		return false
	}
}

func isEscape(s Status) bool {
	switch s {
	case StatusFailed, StatusCancelled, StatusRolledBack, StatusNeedsManualIntervention:
		return true
	default:
		return false
	}
}

var forward = map[Status][]Status{
	StatusCreated:             {StatusCollectingContext},
	StatusCollectingContext:   {StatusPlanning},
	StatusPlanning:            {StatusAwaitingApproval, StatusExecuting},
	StatusAwaitingApproval:    {StatusExecuting},
	StatusExecuting:           {StatusVerifying},
	StatusVerifying:           {StatusEvaluatingSafety},
	StatusEvaluatingSafety:    {StatusAwaitingHumanReview, StatusPublishing},
	StatusAwaitingHumanReview: {StatusPublishing},
	StatusPublishing:          {StatusLearning},
	StatusLearning:            {StatusDone},
}

// Transition 将任务状态推进到 to。
func (s *FileStore) Transition(id string, to Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tk, ok := s.byID[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	cur := s.tasks[tk]
	if !CanTransition(cur.Status, to) {
		return fmt.Errorf("illegal transition %s -> %s", cur.Status, to)
	}
	cur.Status = to
	cur.UpdatedAt = s.now()
	s.tasks[tk] = cur
	return s.saveLocked()
}
