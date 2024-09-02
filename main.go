package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	kanister "github.com/kanisterio/kanister/pkg"
	kancr "github.com/kanisterio/kanister/pkg/apis/cr/v1alpha1"
	"github.com/kanisterio/kanister/pkg/blueprint/validate"
	"github.com/kanisterio/kanister/pkg/function"
	yaml "gopkg.in/yaml.v2"
)

const API_VERSION = "v1"
const PORT = "8080"

const (
	ParticipantKanister     = "Kanister"
	ParticipantAppNamespace = "App"
	ActorUser               = "User"
)

type SequenceData struct {
	Participants []string `json:"participants"`
	Actors       []string `json:"actors"`
	Actions      []Action `json:"actions"`
}

type Action struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Phases      []Phase `json:"phases"`
}

type Phase struct {
	Description string
	Messages    []Message `json:"messages"`
}

type Message struct {
	CreateParticipant  bool
	DestroyParticipant bool
	From               string `json:"from"`
	To                 string `json:"to"`
	Action             string `json:"action"`
	Note               string `json:"note"`
	ArrowType          string
}

func main() {
	log.Printf("server started accepting requests on port=%s..\n", PORT)
	http.HandleFunc(fmt.Sprintf("/%s/validate", API_VERSION), HandleValidator)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func HandleValidator(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	data, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, fmt.Sprintf("Bad Request. Error: %s", err.Error()), http.StatusBadRequest)
		return
	}
	bp := kancr.Blueprint{}
	if err := yaml.Unmarshal(data, &bp); err != nil {
		log.Println(err)
		http.Error(w, fmt.Sprintf("Bad Request. Error: %s", err.Error()), http.StatusBadRequest)
		return
	}
	fmt.Println("Request::", string(data))
	fmt.Printf("BP Object::\n%+v\n", bp)
	err1 := validate.Do(&bp, kanister.DefaultVersion)
	if err1 != nil {
		log.Printf("Failed to validate blueprint, %v\n", err1)
		http.Error(w, fmt.Sprintf("Failed to validate Blueprint. Error: %s", err1), http.StatusBadRequest)
		return
	}

	seq, err := sequenceRelation(bp)
	if err != nil {
		log.Print("error:", err)
		http.Error(w, fmt.Sprintf("Failed to create flowchart for Blueprint. Error: %s", err), http.StatusNotImplemented)

	}
	io.WriteString(w, seq)
}

func sequenceRelation(bp kancr.Blueprint) (string, error) {
	data := SequenceData{
		Participants: []string{ParticipantKanister},
		Actors:       []string{ActorUser},
	}
	for actionName, action := range bp.Actions {
		seqAction := Action{Title: actionName}
		for i, phase := range action.Phases {
			seqAction.Phases = append(seqAction.Phases, Phase{Description: "phase " + phase.Name})
			message, err := messageForKanisterFunc(phase)
			if err != nil {
				return "", err
			}
			seqAction.Phases[i].Messages = append(seqAction.Phases[i].Messages, message...)
		}
		data.Actions = append(data.Actions, seqAction)
	}
	mermaidSyntax := generateMermaidSyntax(data)
	log.Print("mermaidSyntax::", mermaidSyntax)
	return mermaidSyntax, nil
}

func generateMermaidSyntax(data SequenceData) string {
	syntax := "sequenceDiagram\n"
	for _, actor := range data.Actors {
		syntax += "    actor " + actor + "\n"
	}
	for _, participant := range data.Participants {
		syntax += "    participant " + participant + "\n"
	}
	for _, act := range data.Actions {
		syntax += fmt.Sprintf("    %s->>%s: %s\n", ActorUser, ParticipantKanister, act.Title)
		for _, phase := range act.Phases {
			syntax += fmt.Sprintf("    note right of %s: %s\n", ParticipantKanister, phase.Description)
			for _, msg := range phase.Messages {
				if msg.CreateParticipant {
					syntax += "    create participant " + msg.To + "\n"
				}
				if msg.DestroyParticipant {
					syntax += "    destroy " + msg.From + "\n"
				}
				syntax += "    " + msg.From + msg.ArrowType + msg.To + ": " + msg.Action + "\n"
				if msg.Note != "" {
					syntax += fmt.Sprintf("    note right of %s: %s\n", msg.To, msg.Note)
				}
			}
		}
		syntax += fmt.Sprintf("    %s-->>%s: %s\n", ParticipantKanister, ActorUser, act.Title+" completed!")
	}
	return syntax
}

func messageForKanisterFunc(phase kancr.BlueprintPhase) ([]Message, error) {
	switch phase.Func {
	case function.KubeTaskFuncName:
		appNamespace := phase.Args[function.KubeTaskNamespaceArg].(string)
		if appNamespace == "" || strings.HasPrefix(appNamespace, "{{") {
			appNamespace = ParticipantAppNamespace
		}
		return []Message{
			{
				CreateParticipant: true,
				From:              ParticipantKanister,
				To:                fmt.Sprintf("%s/kanister-job", appNamespace),
				Action:            phase.Func,
				Note:              fmt.Sprintf("Create a tooling pod with <br> image: %s <br> namespace: %s <br> and execute commands", phase.Args[function.KubeTaskImageArg], appNamespace),
				ArrowType:         "->>",
			},
			{
				DestroyParticipant: true,
				From:               fmt.Sprintf("%s/kanister-job", appNamespace),
				To:                 ParticipantKanister,
				Action:             "Done",
				ArrowType:          "-->>",
			},
		}, nil

	case function.ScaleWorkloadFuncName:
		appNamespace := phase.Args[function.ScaleWorkloadNamespaceArg].(string)
		if appNamespace == "" || strings.HasPrefix(appNamespace, "{{") {
			appNamespace = ParticipantAppNamespace
		}
		workload := phase.Args[function.ScaleWorkloadNameArg]
		kind := phase.Args[function.ScaleWorkloadKindArg]
		count := phase.Args[function.ScaleWorkloadReplicas]
		return []Message{
			{
				CreateParticipant: true,
				From:              ParticipantKanister,
				To:                fmt.Sprintf("%s/%s/%s", appNamespace, kind, workload),
				Action:            phase.Func,
				Note:              fmt.Sprintf("Set the replica count <br> of %s/%s to %v", kind, workload, count),
				ArrowType:         "->>",
			},
			{
				DestroyParticipant: true,
				From:               fmt.Sprintf("%s/%s/%s", appNamespace, kind, workload),
				To:                 ParticipantKanister,
				Action:             "Done",
				ArrowType:          "-->>",
			},
		}, nil

	default:
		return nil, fmt.Errorf("support for function %s not implemented yet", phase.Func)
	}
}
