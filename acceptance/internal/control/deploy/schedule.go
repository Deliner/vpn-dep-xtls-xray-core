package deploy

import "container/heap"

func (queue nodeQueue) Len() int           { return len(queue) }
func (queue nodeQueue) Less(i, j int) bool { return queue[i] < queue[j] }
func (queue nodeQueue) Swap(i, j int)      { queue[i], queue[j] = queue[j], queue[i] }
func (queue *nodeQueue) Push(value any)    { *queue = append(*queue, value.(int)) }

func (queue *nodeQueue) Pop() any {
	last := len(*queue) - 1
	value := (*queue)[last]
	*queue = (*queue)[:last]
	return value
}

func newSchedule(order []string, dependencies map[string][]string) deploymentSchedule {
	schedule := deploymentSchedule{order: order, waiting: map[string]int{}, dependents: map[string][]int{}}
	for index, id := range order {
		schedule.waiting[id] = len(dependencies[id])
		if len(dependencies[id]) == 0 {
			schedule.ready = append(schedule.ready, index)
		}
		for _, downstream := range dependencies[id] {
			schedule.dependents[downstream] = append(schedule.dependents[downstream], index)
		}
	}
	heap.Init(&schedule.ready)
	return schedule
}

func (schedule *deploymentSchedule) take(limit int) []string {
	var nodes []string
	for len(nodes) < limit && len(schedule.ready) > 0 {
		nodes = append(nodes, schedule.order[heap.Pop(&schedule.ready).(int)])
	}
	return nodes
}

func (schedule *deploymentSchedule) complete(id string) {
	for _, index := range schedule.dependents[id] {
		next := schedule.order[index]
		schedule.waiting[next]--
		if schedule.waiting[next] == 0 {
			heap.Push(&schedule.ready, index)
		}
	}
}
