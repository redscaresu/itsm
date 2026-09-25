package jev_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/jev"
	"github.com/redscaresu/itsm/internal/jev/jevfake"
)

// wantFirstRequest is HLD 4.1 (request and state) plus 4.2 (questions),
// copied from the HLD by hand. It is the contract; do not generate it.
const wantFirstRequest = `{
  "model": "jev-latest",
  "state": {
    "ticket": {
      "title": "Leeds office network down",
      "description": "Nobody in Leeds office can get on the network since 9am. Wired and Wi-Fi both dead.",
      "follow_up_answers": []
    },
    "context": {
      "organisation": "Probation Service, England and Wales. Staff work from probation offices; some services are national.",
      "teams": {
        "network": "connectivity, VPN, Wi-Fi, whole-site outages, and changes to network kit: firewall rules, ports, switches, cabling",
        "end_user_compute": "laptops, desktops, printers, phones, peripherals a person uses directly",
        "applications": "case-management and business applications such as nDelius, OASys, and shared drives",
        "identity_access": "passwords, accounts, MFA, permissions, access to systems",
        "service_desk": "anything that fits none of the above, or is too vague to place"
      }
    }
  },
  "questions": {
    "team": {
      "type": "choice",
      "instructions": "Which support team should handle ` + "`ticket`" + `? Use ` + "`context.teams`" + ` for what each team covers. If the ticket is too vague to place, or fits none of the teams, choose service_desk.",
      "criteria": {
        "network": "connectivity, VPN, Wi-Fi, a whole site losing connection, or a change to network kit such as firewall rules, ports, switches or cabling",
        "end_user_compute": "a laptop, desktop, printer, phone or other device a person uses directly",
        "applications": "a business application or case system such as nDelius or OASys, or a shared drive",
        "identity_access": "passwords, account lockouts, MFA, permissions or access to a system",
        "service_desk": "none of the above fits, or the ticket is too vague to tell"
      }
    },
    "kind": {
      "type": "choice",
      "instructions": "What kind of ticket is ` + "`ticket`" + `?",
      "criteria": {
        "fault": "something that used to work is broken, down or degraded",
        "change": "a request to add, change or remove something in a setup that is working: a port, a rule, a permission, a new device or account",
        "question": "a how-to question, a request for advice, or a cosmetic issue that stops no one"
      }
    },
    "scope": {
      "type": "choice",
      "instructions": "How widely is the problem in ` + "`ticket`" + ` felt? Use ` + "`ticket.follow_up_answers`" + ` if present.",
      "criteria": {
        "one_user": "only the reporter, or one named person, is affected",
        "one_office": "a single probation office, site or building is affected",
        "region": "several offices in one area or region are affected",
        "national": "every office is affected, or everyone who uses a national service is; not one person's or one office's problem with a national system",
        "unclear": "the ticket does not say, and it cannot be told, how widely the problem is felt"
      }
    },
    "blocked": {
      "type": "noul",
      "instructions": "Is at least one person unable to do their work because of the problem in ` + "`ticket`" + `?",
      "criteria": {
        "true": "someone cannot do a task at all until this is fixed",
        "false": "work continues, perhaps more slowly or with a workaround, or nothing is broken"
      }
    },
    "states_location": {
      "type": "noul",
      "instructions": "Does ` + "`ticket`" + ` say which office, site or region is affected, or make clear that the problem is national or only affects the reporter? Count ` + "`ticket.follow_up_answers`" + ` as part of the ticket."
    },
    "states_service": {
      "type": "noul",
      "instructions": "Does ` + "`ticket`" + ` name the system, application, device or service that has the problem? Count ` + "`ticket.follow_up_answers`" + ` as part of the ticket."
    }
  }
}`

// Catches: a renamed question ID or option, reworded criteria, a changed
// model name, or follow_up_answers sent as null — jev would silently answer
// a different question.
func TestJudgeRequestBodyMatchesHLD(t *testing.T) {
	fake := jevfake.New()
	defer fake.Close()
	c := jev.NewClient(jev.Config{BaseURL: fake.URL, APIKey: "test-key"})

	_, err := c.Judge(context.Background(), jev.NewState(
		"Leeds office network down",
		"Nobody in Leeds office can get on the network since 9am. Wired and Wi-Fi both dead.",
		nil,
	))
	require.NoError(t, err)

	reqs := fake.Requests()
	require.Len(t, reqs, 1)
	assert.JSONEq(t, wantFirstRequest, string(reqs[0].Body))
	assert.Equal(t, "Bearer test-key", reqs[0].Authorization)
}
