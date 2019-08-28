package main

import (
	"context"
	"fmt"
	"github.com/shomali11/slacker"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Lock struct {
	owner    string
	resource string
	expiry   time.Time
}

// Unlocked resources own themself, because why not
func unlocked(res string) Lock {
	return Lock{res, res, time.Time{}}
}

func (l Lock) IsUnlocked() bool {
	return l.owner == l.resource && l.expiry.IsZero()
}

func main() {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		panic(err)
	}

	bot := slacker.NewClient(os.Getenv("SLACK_TOKEN"), slacker.WithDebug(true))
	locks := map[string]Lock{}
	var m sync.Mutex

	lockDefinition := &slacker.CommandDefinition{
		Description: "Lock a resource by name for a specified duration (minutes)",
		Example:     "lock HOST_A 10",
		Handler: func(request slacker.Request, response slacker.ResponseWriter) {
			resource := strings.ToLower(request.StringParam("resource", ""))
			duration := request.IntegerParam("number", 60)
			user := request.Event().User
			targetExpiry := time.Now().Add(time.Duration(duration) * time.Minute)
			if resource == "" {
				response.Reply("Please provide a resource name as the first argument")
				return
			}
			//TODO: sanitize inputs
			m.Lock()
			defer m.Unlock()
			if l, ok := locks[resource]; !ok || l.expiry.Before(time.Now()) {
				locks[resource] = Lock{user, resource, targetExpiry}
				timeStr := targetExpiry.In(location).Format("Mon 15:04")
				response.Reply(fmt.Sprintf("Locked %s until %s", resource, timeStr))
			} else {
				response.Reply(fmt.Sprintf("Resource %s is currently locked by %s for the next %s", resource, l.owner, time.Until(l.expiry)))
			}
		},
	}

	unlockDefinition := &slacker.CommandDefinition{
		Description: "Unlock a resource that you have locked",
		Example:     "unlock HOST_A",
		Handler: func(request slacker.Request, response slacker.ResponseWriter) {
			resource := strings.ToLower(request.StringParam("resource", ""))
			force := strings.ToLower(request.StringParam("force", ""))
			user := request.Event().User
			if resource == "" {
				response.Reply("Please provide a resource name as the first argument")
				return
			}
			m.Lock()
			defer m.Unlock()
			if l, ok := locks[resource]; ok {
				if l.owner != user && force != "force" {
					response.Reply("You don't own the lock on that resource. (Add 'force' and try again.)")
					return
				}
				delete(locks, resource)
				response.Reply("Resource has been unlocked.")
			} else {
				response.Reply("Resource was not locked.")
			}
		},
	}

	statusDefinition := &slacker.CommandDefinition{
		Description: "Return lock statuses for all currently-locked resources",
		Example:     "status",
		Handler: func(request slacker.Request, response slacker.ResponseWriter) {
			m.Lock()
			defer m.Unlock()
			if l, ok := locks[resource]; ok {
				if l.owner != user && force != "force" {
					response.Reply("You don't own the lock on that resource. (Add 'force' and try again.)")
					return
				}
				delete(locks, resource)
				response.Reply("Resource has been unlocked.")
			} else {
				response.Reply("Resource was not locked.")
			}
		},
	}

	bot.Command("lock <resource> <number>", lockDefinition)
	bot.Command("unlock <resource>", unlockDefinition)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = bot.Listen(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
