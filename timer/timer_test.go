package timer

import (
	"errors"
	"sync"
	"testing"
)

func TestPostitiveDuration(t *testing.T) {
	at := NewEndToEndTimer("foo")
	at.Stop(nil)
	if at.Duration == 0 {
		t.Fail()
	}

}

func TestContributors(t *testing.T) {
	at := NewEndToEndTimer("foo")
	c1 := at.StartContributor("c1")
	c2 := at.StartContributor("c2")
	c2.End(nil)
	c1.End(nil)
	at.Stop(nil)

	if at.Error != "" {
		t.Fail()
	}

	if c1.Duration <= 0 || c2.Duration <= 0 {
		t.Fail()
	}

	if at.ErrorFree == false {
		t.Fail()
	}
}

func TestIfContributorErrorsThenTimerErrors(t *testing.T) {
	at := NewEndToEndTimer("foo")
	c1 := at.StartContributor("c1")
	c2 := at.StartContributor("c2")
	c2.End(errors.New("oh whoops"))
	c1.End(nil)
	at.Stop(nil)

	if at.Error != "" {
		t.Fail()
	}

	if len(at.ContributorErrors()) != 1 {
		t.Fail()
	}

	if at.ErrorFree == true {
		t.Fail()
	}

}

func TestMultiBackendRecordings(t *testing.T) {
	at := NewEndToEndTimer("foo")
	c1 := at.StartContributor("c1")
	c2 := at.StartContributor("c2")
	c3 := at.StartContributor("c3")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		be1 := c3.StartServiceCall("workflo", "localhost:12345")
		be1.End(nil)
	}()

	go func() {
		defer wg.Done()
		be2 := c3.StartServiceCall("doc munger", "localhost:12345")
		be2.End(nil)
	}()

	wg.Wait()

	c3.End(nil)

	c2.End(nil)
	c1.End(nil)
	at.Stop(nil)

	if at.Error != "" {
		t.Fail()
	}

	if c1.Duration <= 0 || c2.Duration <= 0 || c3.Duration <= 0 {
		t.Fail()
	}

	if at.ErrorFree == false {
		t.Fail()
	}

	if len(c3.ServiceCalls) != 2 {
		t.Fail()
	}

	println(at.ToJSONString())
}
