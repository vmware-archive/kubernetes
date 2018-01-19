package bootstrap

import (
	"sync"
)

var once sync.Once
var waiting = make(chan bool)

func Bootstrap() {
	done := make(chan bool)
	go func() {
		once.Do(bootstrapOnce)
		<- waiting
		done <- true
	}()
	<- done
}

func bootstrapOnce() {
	// TBD
	// 1. Read vSphere conf and get VSphere instances
	// 2. Get Node to VSphere mapping
	// 3. Set NodeMapper in vSphere context
	TestContext = Context{}
	close(waiting)
}