package wrapper

type clusterUpdateState struct {
	failuresAllow int
	ch            chan int
	state         int
	counter       int
}

func newClusterUpdateState(failuresAllow int) *clusterUpdateState {
	return &clusterUpdateState{
		failuresAllow: failuresAllow,
		ch:            make(chan int, 1),
	}
}

func (c *clusterUpdateState) clearState() {
	c.counter = 0
	c.state = 0
	for {
		select {
		case <-c.ch:
		default:
			return
		}
	}
}

func (c *clusterUpdateState) setState(state int) {
	if state != c.state {
		c.state = state
		c.counter = 0
	} else {
		c.counter++
	}

	if c.counter >= c.failuresAllow {
		c.clearState()
		c.ch <- state
	}
}
