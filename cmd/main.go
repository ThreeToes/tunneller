package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/aws/aws-sdk-go/service/rds"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	log "github.com/sirupsen/logrus"
	"github.com/threetoes/tunneller/internal"
)

func main() {
	profileF := flag.String("profile", "", "Name of the profile to use")
	localPortF := flag.Int("local-port", -1, "Port to use")
	regionF := flag.String("region", "", "AWS Region")
	helpF := flag.Bool("help", false, "Display help and exit")
	usernameF := flag.String("username", "ec2-user", "EC2 instance username")

	flag.Parse()

	if *helpF {
		flag.Usage()
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Cannot find AWS credentials file")
		os.Exit(1)
	}
	joined := path.Join(home, ".aws/credentials")
	fmt.Printf("Reading config from %s\n", joined)

	prof := internal.NewIniConfig(joined)
	if err = prof.Refresh(); err != nil {
		log.Fatalf("Could not load profiles: %v", err)
	}
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	statusLabel := widgets.NewParagraph()
	optionsList := widgets.NewList()
	optionsList.TextStyle = ui.NewStyle(ui.ColorYellow)
	ui.Clear()
	statusLabel.Text = "Loading"
	ui.Render(statusLabel)

	statusLabel.Text = "Choose a region"
	var options []string
	options = []string{
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"af-south-1",
		"ap-east-1",
		"ap-south-1",
		"ap-northeast-2",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-northeast-1",
		"ca-central-1",
		"eu-central-1",
		"eu-west-1",
		"eu-west-2",
		"eu-south-1",
		"eu-west-3",
		"eu-north-1",
		"me-south-1",
		"sa-east-1",
	}
	var selectedRegion string
	if *regionF == "" {
		optionsList.Rows = options
		if handleListSelect(statusLabel, optionsList) {
			return
		}
		selectedRegion = options[optionsList.SelectedRow]
	} else {
		selectedRegion = *regionF
	}

	var port int
	if *localPortF == -1 {
		port = 8888
	} else {
		port = *localPortF
	}

	options = nil
	profileContainers := prof.GetProfiles()
	var selectedProfile internal.ProfileContainer
	for i, p := range profileContainers {
		if p.GetName() == *profileF {
			selectedProfile = p
			break
		}
		options = append(options, fmt.Sprintf("[%d] %s", i, p.GetName()))
	}

	if selectedProfile == nil {
		optionsList.Rows = options
		statusLabel.Text = "Choose a profile"
		if handleListSelect(statusLabel, optionsList) {
			return
		}
		selectedProfile = profileContainers[optionsList.SelectedRow]
		statusLabel.Text = fmt.Sprintf("Chose profile %s. Connecting", selectedProfile.GetName())
		ui.Clear()
		ui.Render(statusLabel)
	}


	if err = selectedProfile.Connect(selectedRegion); err != nil {
		statusLabel.Text = fmt.Sprintf("Error connecting profile to region %s: %v", selectedRegion, err)
		ui.Render(statusLabel)
		ui.Close()
		log.Fatalf(statusLabel.Text)
	}

	statusLabel.Text = fmt.Sprintf("Connected, fetching EC2 instances...")
	ui.Clear()
	ui.Render(statusLabel)
	ecSvc, err := selectedProfile.GetEC2Service()
	if err != nil {
		ui.Close()
		log.Fatalf("Could not get EC2 instances: %v", err)
	}
	var instances []*ec2.Instance
	var nextToken string
	for {
		var nt *string
		if nextToken != "" {
			nt = aws.String(nextToken)
		}
		resp, err := ecSvc.DescribeInstances(&ec2.DescribeInstancesInput{
			DryRun:     aws.Bool(false),
			MaxResults: aws.Int64(20),
			NextToken:  nt,
		})

		if err != nil {
			statusLabel.Text = fmt.Sprintf("Could not describe instances: %v", err.Error())
			ui.Render(statusLabel)
			time.Sleep(3 * time.Second)
			ui.Close()
			log.Fatalf("Could not describe instances: %v", err.Error())
		}
		for _, res := range resp.Reservations {
			for _, instance := range res.Instances {
				instances = append(instances, instance)
			}
		}
		if resp.NextToken == nil {
			break
		}
		nextToken = *resp.NextToken
	}
	statusLabel.Text = fmt.Sprintf("Got %d instances. Please choose below", len(instances))
	options = nil

	for i, inst := range instances {
		var tags []string
		for _, t := range inst.Tags {
			tags = append(tags, fmt.Sprintf("%s=%s", *t.Key, *t.Value))
		}
		formatted := fmt.Sprintf("[%d] %s \t %s", i, *inst.InstanceId, strings.Join(tags, ","))
		options = append(options, formatted)
	}
	optionsList.Rows = options
	ui.Render(statusLabel, optionsList)
	if handleListSelect(statusLabel, optionsList) {
		return
	}
	selectedBastion := instances[optionsList.SelectedRow]
	statusLabel.Text = fmt.Sprintf("Selected %s as the bastion. Getting RDS servers", *selectedBastion.InstanceId)
	ui.Clear()
	ui.Render(statusLabel)
	dbSvc, err := selectedProfile.GetRDSService()
	if err != nil {
		ui.Close()
		log.Fatalf("Could not initialise RDS service")
	}
	dbResp, err := dbSvc.DescribeDBInstances(&rds.DescribeDBInstancesInput{})
	if err != nil {
		ui.Close()
		log.Fatalf("Could not initialise RDS service")
	}
	dbs := dbResp.DBInstances
	options = nil
	for i, d := range dbs {
		options = append(options, fmt.Sprintf("[%d] %s", i, *d.Endpoint.Address))
	}
	optionsList.Rows = options
	if handleListSelect(statusLabel, optionsList) {
		return
	}
	selectedDb := dbs[optionsList.SelectedRow]
	ui.Clear()
	statusLabel.Text = fmt.Sprintf("Chose %s. Tunnelling in", *selectedDb.Endpoint.Address)
	ui.Render(statusLabel)
	cnnct, err := selectedProfile.GetEC2InstanceConnectService()
	if err != nil {
		ui.Close()
		log.Fatalf("Could not get ec2instanceconnect session: %v", err)
	}

	ec2Endpoint, err := internal.NewEC2Endpoint(*selectedBastion.InstanceId, ecSvc, cnnct)
	if err != nil {
		ui.Close()
		log.Fatalf("Could not configure bastion endpoint: %v", err)
	}
	config, err := ec2Endpoint.GetSSHConfig()
	if err != nil {
		ui.Close()
		log.Fatalf("Could not get bastion config: %v", err)
	}
	sshSess, err := ssh.Dial("tcp", ec2Endpoint.String(), config)
	if err != nil {
		ui.Close()
		log.Fatalf("Could not dial bastion: %v", err)
	}

	defer sshSess.Close()

	statusLabel.Text = "Connected to bastion, starting tunnel"
	ui.Clear()
	ui.Render(statusLabel)
	dbEndpoint := internal.NewEndpoint(fmt.Sprintf("%s@%s:%d",
		*usernameF,
		*selectedDb.Endpoint.Address, *selectedDb.Endpoint.Port))
	done, err := internal.Tunnel(port, dbEndpoint, ec2Endpoint)
	if err != nil {
		ui.Close()
		log.Fatalf("Could start local listener: %v", err)
	}
	statusLabel.Text = fmt.Sprintf("Tunnel started. Connect on localhost port %d with your DB client using regular credentials. Press Ctrl-C to end", port)
	termWidth, termHeight := ui.TerminalDimensions()
	statusLabel.SetRect(((termWidth / 2) - 10), ((termHeight / 2) - 5),
		((termWidth / 2) + 10), ((termHeight / 2) + 5))
	ui.Clear()
	ui.Render(statusLabel)
	evt := ui.PollEvents()
	for {
		select {
		case e := <-evt:
			if e.ID == "<C-c>" {
				ui.Clear()
				ui.Close()
				log.Infof("Shutting down listener thread")
				done <- 1
				log.Infof("Thanks, goodbye")
				os.Exit(0)
			}
			ui.Clear()
			termWidth, termHeight = ui.TerminalDimensions()
			statusLabel.SetRect(((termWidth / 2) - 15), ((termHeight / 2) - 15),
				((termWidth / 2) + 15), ((termHeight / 2) + 15))
			ui.Render(statusLabel)
		case <-done:
			log.Println("Tunnel server reports it's had an error. Exiting")
			os.Exit(1)
		}
	}
}

func handleKeyboardInput(prompt string) (string, bool) {
	statusLabel := widgets.NewParagraph()
	inputBox := widgets.NewParagraph()
	statusLabel.Text = prompt
	uiEvents := ui.PollEvents()
	builder := strings.Builder{}
	for {
		termWidth, _ := ui.TerminalDimensions()
		statusLabel.SetRect(0, 0, termWidth, 4)
		inputBox.SetRect(0, 3, termWidth, 4)
		inputBox.Text = builder.String()
		ui.Clear()
		ui.Render(statusLabel, inputBox)
		e := <-uiEvents
		switch {
		case e.ID == "<C-c>":
			return "", true
		case e.ID == "<Enter>":
			return builder.String(), false
		case strings.HasPrefix(e.ID, "<C-"):
			continue
		default:
			builder.WriteString(e.ID)
		}
	}
	return builder.String(), false
}

func handleListSelect(statusLabel *widgets.Paragraph, optionsList *widgets.List) bool {
	optionsList.SelectedRow = 0
	uiEvents := ui.PollEvents()
	for {
		termWidth, termHeight := ui.TerminalDimensions()
		statusLabel.SetRect(0, 0, termWidth, 1)
		optionsList.SetRect(0, 1, termWidth, termHeight)
		ui.Render(statusLabel, optionsList)
		e := <-uiEvents
		switch e.ID {
		case "<C-c>":
			return true
		case "j", "<Down>":
			optionsList.ScrollDown()
		case "k", "<Up>":
			optionsList.ScrollUp()
		case "<C-d>":
			optionsList.ScrollHalfPageDown()
		case "<C-u>":
			optionsList.ScrollHalfPageUp()
		case "<C-f>":
			optionsList.ScrollPageDown()
		case "<C-b>":
			optionsList.ScrollPageUp()
		case "<Home>":
			optionsList.ScrollTop()
		case "G", "<End>":
			optionsList.ScrollBottom()
		case "<Enter>", "<Space>":
			return false
		}
	}
	return false
}
