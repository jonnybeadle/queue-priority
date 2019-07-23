package main

import (
	"container/heap"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"time"
)

var sla30m, _ = time.ParseDuration("30m")
var sla5m, _ = time.ParseDuration("5m")

// hold information about each conversation
type Conversation struct {
	id    string
	vel   int64
	sla   time.Duration
	tiq   time.Duration
	score int64
	index int //index in the heap
}

type Queue []*Conversation

func (q Queue) Len() int { return len(q) }

func (q Queue) Less(i, j int) bool {
	return q[i].score < q[j].score
}

func (q *Queue) Swap(i, j int) {
	a := *q
	a[i], a[j] = a[j], a[i]
	a[i].index = i
	a[j].index = j

}

type byScore []*Conversation

func (s byScore) Len() int {
	return len(s)
}

func (s byScore) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byScore) Less(i, j int) bool {
	return s[i].score > s[j].score
}

func (q *Queue) Push(x interface{}) {
	a := *q // temp var from the queue
	//p := fmt.Println
	n := len(a)
	//p(n)
	c := x.(*Conversation)
	a = a[0 : n+1] // assign new slice +1 bigger
	a[n] = c       // put new Conversation at back of queue
	*q = a         // assign new queue back to the pointer ref *q
}

func (q *Queue) Pop() interface{} {
	a := *q
	*q = a[0 : len(a)-1] // make queue -1 smaller
	c := a[0]            // FILO - get one at the front as we order by SCORE and take from the top of the queue when needed
	// c := a[len(a)-1] // FIFO
	fmt.Println(">>> POP c: ", c.id)
	return c
}

type MsgQ struct {
	queue Queue
	in    chan *Conversation
	out   chan *Conversation
	updates chan string
	done  chan string
	index int
}

func (mq *MsgQ) Receive() {
	// heap.Push(&mq.queue, c)
	for c := range mq.in {
		runes := []rune(c.id)
		cid := string(runes[0:8])
		fmt.Printf("\n\n >>> mq.in <- [new conversation] %+v \n\n", cid)
		
		heap.Push(&mq.queue, c)
		mq.updates <- fmt.Sprintf("Conversation Added [%v]",cid)
	}
}

func (mq *MsgQ) Release() {
	//fmt.Printf("\n\n___ RELEASE ... waiting on mq.out <- from current queue of lenth := (%v) items\n\n", len(mq.queue))
	for c := range mq.out {

		runes := []rune(c.id)
		cid := string(runes[0:8])
		fmt.Printf("\n\n>>> mq.out <- Remove Conv <- [%v] [%v] ~%v\n\n", c.index, cid, c.score)
		if c.index == 0 && len(mq.queue) == 1 {
			//fmt.Println("\n\n --- index 0 points to ... ", mq.queue[c.index])
			mq.queue = nil
		} else if c.index == 0 && len(mq.queue) > 1 {
			//fmt.Println("\n\n --- index 0 points to ... ", mq.queue[c.index], len(mq.queue))
			mq.queue = mq.queue[1:]
		}
		if len(mq.queue) > c.index+1 {

			mq.queue[c.index], mq.queue[len(mq.queue)-1] = mq.queue[len(mq.queue)-1], mq.queue[c.index]
			mq.queue = mq.queue[:len(mq.queue)-1] // Truncate slice.
			// heap.Init(&mq.queue)
		}
		msg := fmt.Sprintf("Conversation Removed [%v]",cid)
		mq.updates <- msg
	}
}

func (mq *MsgQ) Len() int {
	return len(mq.queue)
}

func (mq *MsgQ) Process() {
	// range over Queue items and score each one, then sort again byScore
	for _, c := range mq.queue {
		c.tiq = (time.Duration(c.tiq) + (time.Minute * 5)) // add 5mins to each conversation's time in queue value
		c.Score()
		if c.score > 2000 {
			mq.out <- c // remove conv by sending to the .out chan for removal
		} 
	}
	sort.Sort(byScore(mq.queue)) // sort by score
}

type Batch struct {
}

func random(min, max int) int {

	r := rand.Intn(max-min) + min
	return r
}

func PopulateQ(size int) *MsgQ {

	incoming := make(chan *Conversation)
	outgoing := make(chan *Conversation)
	updates := make(chan string)
	done := make(chan string, 1)

	mq := &MsgQ{make(Queue, 0, size*2), incoming, outgoing, updates, done, 0}
	// populate the queue with random conversations
	for i := 0; i < size; i++ {
		mins := fmt.Sprintf("%vm", random(1, 30))
		//fmt.Println(">> mins : ", mins)
		t, _ := time.ParseDuration(mins)
		//fmt.Println("--- Random time in q ==", t)
		c := &Conversation{
			id:    generateConversationId(),
			vel:   1,
			tiq:   t,
			sla:   sla30m,
			score: 0,
		}
		//fmt.Println("*** new conv *** ", c)
		heap.Push(&mq.queue, c)
	}
	go mq.Receive()
	go mq.Release()
	go func() {
			for m:= range mq.updates {
				fmt.Printf("\n! mq.update <- m '%v'\n\n", m)
				//mq.Display()
		}
	}()

	go mq.Monitor()
	return mq
}

func (mq *MsgQ) Monitor() {

	ticker := time.NewTicker(2 * time.Second)
	startTime := time.Now().Local()
	var minsToAdd = 5
	for range ticker.C {
		// startTime.Add(time.Minute * 10)
		startTime.Add(time.Duration(minsToAdd) * time.Minute)
		//p("[monitor] vclock := ", vClock)
		fmt.Printf("...clock tick...%v\n", startTime.Add(time.Duration(minsToAdd)*time.Minute))
		mq.Process()
		mq.Display()
		minsToAdd = minsToAdd + 5
		// if queue length is 0 then we are done!
		if len(mq.queue) == 0 {
			
			mq.done <- `--QUEUE IS EMPTY--`
			
		} else {
			fmt.Printf("[%v] item(s) left in the queue!\n\n", len(mq.queue))
		}
	}
	


}

func (mq *MsgQ) Display() {
	p := fmt.Println
	pf := fmt.Printf
	p("======================= Messaging Queue ====================")
	pf("pos \t TIQ \t\t SLA \t score \t VEL \t convId\n\n")
	p("------------------------------------------------------------")
	for i, c := range mq.queue {
		pf("[%v] \t %.fm \t\t %v \t %v \t %v \t %v \n\n", i+1, c.tiq.Minutes(), c.sla, c.score, c.vel, c.id)
	}
	p("======================= =================================== ")
	p("")
}

func (c *Conversation) Score() {
	var cs int64
	now := time.Now()
	tiq := now.Add(-c.tiq)
	//tiq := now.Add(-waitingFor)
	now_ms := now.Unix() * 1000
	tiq_ms := tiq.Unix() * 1000
	cs = ((now_ms - tiq_ms) * int64(c.vel)) / int64(c.sla.Seconds())

	/*
		CS = Conversation Score
		Now = time in ms
		TIQ = time in queue in ms
		Veloctiy = 1-20
		SLA = skill SLA in seconds

		e.g
		n = 1562762345000
		TIQ =

		CS = (Now - TIQ) * Velocity / SLA
	*/

	c.score = cs

}

func generateConversationId() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	//fmt.Println(uuid)
	return uuid
}

func main() {
	p := fmt.Println
	pf := fmt.Printf
	rand.Seed(time.Now().Unix())
	/*
		c2 at vel 2 and wait 10m = 666
		c3 at vel 20 and wait 1m = 666

		why is this? BECAUSE you have INCREASED the vel by x10 BUT DECREASED the wait time by x10 = same result!!!
	*/
	newQ := PopulateQ(10)
	heap.Init(&newQ.queue)

	botConv := &Conversation{
		id:    generateConversationId(),
		vel:   2,
		sla:   time.Duration(30) * time.Minute,
		tiq:   (time.Minute * 1),
		score: 0,
	}
	time.Sleep(5 * time.Second)
	pf("\n\n >>> newQ.in <- botConv vel 20 adding [%v]... ", botConv.id)
	newQ.in <- botConv

	go func() {
		// loop 3 times, create new conversation and wait a random 1-10 sec interval before pushing it down the "in" channel of the Q
		for i := 0; i < 3; i++ {
			waitFor := random(1, 10)
			time.Sleep(time.Duration(waitFor) * time.Second)
			t, _ := time.ParseDuration("1m")
			c := &Conversation{
				id:    generateConversationId(),
				vel:   int64(i + 1),
				tiq:   t,
				sla:   sla30m,
				score: 0,
			}
			newQ.in <- c
		}
	}()

	fmt.Printf("\n\n <- msg received on .done channel! ... %+v\n\n", <-newQ.done)

	// ticker.Stop()
	p("[x] ticker stopped!")
}
