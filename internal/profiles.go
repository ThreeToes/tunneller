package internal

import (
	"fmt"
	"math/rand"

	"github.com/aws/aws-sdk-go/service/ec2instanceconnect"
	"github.com/aws/aws-sdk-go/service/ec2instanceconnect/ec2instanceconnectiface"

	ini "gopkg.in/ini.v1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/pkg/errors"
)

type Profiles interface {
	GetProfiles() []ProfileContainer
	Refresh() error
	GetProfile(profile string) ProfileContainer
}

type iniProfiles struct {
	path     string
	profiles map[string]ProfileContainer
}

func (i *iniProfiles) GetProfiles() []ProfileContainer {
	var profiles []ProfileContainer
	for _, p := range i.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

func (i *iniProfiles) Refresh() error {
	cfg, err := ini.Load(i.path)
	if err != nil {
		return err
	}
	i.profiles = make(map[string]ProfileContainer)

	for _, s := range cfg.Sections() {
		//log.Printf("Discovered profile %s", s.Name())
		if s.HasKey("aws_access_key_id") &&
			s.HasKey("aws_secret_access_key") {
			kId, err := s.GetKey("aws_access_key_id")
			if err != nil {
				return err
			}
			sKey, err := s.GetKey("aws_secret_access_key")
			if err != nil {
				return err
			}
			sp := &secretProfile{
				accessId: kId.Value(),
				secret:   sKey.Value(),
			}
			sp.setName(s.Name())
			i.profiles[s.Name()] = sp
		} else if s.HasKey("role_arn") &&
			s.HasKey("source_profile") {
			roleArn, _ := s.GetKey("role_arn")
			rp := &roleProfile{
				role: roleArn.Value(),
			}
			rp.setName(s.Name())
			i.profiles[s.Name()] = rp
		} else {
			//log.Printf("Could not determine the type of profile %s", s.Name())
		}
	}
	for _, p := range i.GetProfiles() {
		r, ok := p.(*roleProfile)
		if ok {
			source, _ := cfg.Section(r.profileName).GetKey("source_profile")
			v, exists := i.profiles[source.Value()]
			if !exists {
				return fmt.Errorf("Could not create profile %s, source profile %s not found", r.profileName, source.Value())
			}
			r.parent = v
		}
	}
	return nil
}

func (i *iniProfiles) GetProfile(profile string) ProfileContainer {
	v, _ := i.profiles[profile]
	return v
}

func NewIniConfig(path string) Profiles {
	i := &iniProfiles{
		path:     path,
		profiles: nil,
	}
	return i
}

// ProfileContainer provides an interface to get an AWS service
type ProfileContainer interface {
	GetName() string
	Connect(region string) error
	GetEC2Service() (ec2iface.EC2API, error)
	GetRDSService() (rdsiface.RDSAPI, error)
	GetSTSService() (stsiface.STSAPI, error)
	GetEC2InstanceConnectService() (ec2instanceconnectiface.EC2InstanceConnectAPI, error)
}

type baseProfile struct {
	profileName string
	session     *session.Session
}

func (b *baseProfile) setName(n string) {
	b.profileName = n
}

func (b *baseProfile) GetName() string {
	return b.profileName
}

func (b *baseProfile) GetRDSService() (rdsiface.RDSAPI, error) {
	if b.session == nil {
		return nil, fmt.Errorf("Not connected")
	}
	return rds.New(b.session), nil
}

func (b *baseProfile) GetEC2Service() (ec2iface.EC2API, error) {
	if b.session == nil {
		return nil, fmt.Errorf("Not connected")
	}
	return ec2.New(b.session), nil
}

func (b *baseProfile) GetSTSService() (stsiface.STSAPI, error) {
	if b.session == nil {
		return nil, fmt.Errorf("Session not connected, cannot create STS service")
	}
	return sts.New(b.session), nil
}

func (b *baseProfile) GetEC2InstanceConnectService() (ec2instanceconnectiface.EC2InstanceConnectAPI, error) {
	if b.session == nil {
		return nil, fmt.Errorf("Session not connected, cannot create EC2 connect service")
	}
	return ec2instanceconnect.New(b.session), nil
}

type secretProfile struct {
	baseProfile
	accessId string
	secret   string
}

func (s *secretProfile) Connect(region string) error {
	if s.session == nil || *s.session.Config.Region != region {
		creds := credentials.NewCredentials(&credentials.StaticProvider{
			Value: credentials.Value{
				AccessKeyID:     s.accessId,
				SecretAccessKey: s.secret,
			},
		})
		conf := aws.NewConfig()
		conf.Region = aws.String(region)
		conf.Credentials = creds
		var err error
		s.session, err = session.NewSession(conf)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failure to create session for profile %s", s.profileName))
		}
	}
	return nil
}

type roleProfile struct {
	baseProfile
	role   string
	parent ProfileContainer
}

func (r *roleProfile) Connect(region string) error {
	if r.session != nil &&
		*r.session.Config.Region == region {
		return nil
	}
	err := r.parent.Connect(region)
	if err != nil {
		return errors.Wrap(err, "Error trying to connect base profile")
	}
	tok, err := r.parent.GetSTSService()
	if err != nil {
		return errors.Wrap(err, "Could not get STS service from base profile")
	}
	sessionName := fmt.Sprintf("dogenet-wow-%d", rand.Int())
	assumedRole, err := tok.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(r.role),
		RoleSessionName: aws.String(sessionName),
	})
	if err != nil {
		return err
	}
	r.session, err = session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			*assumedRole.Credentials.AccessKeyId,
			*assumedRole.Credentials.SecretAccessKey,
			*assumedRole.Credentials.SessionToken),
		Region: aws.String(region),
	})
	if err != nil {
		return errors.Wrap(err, "Error creating session")
	}
	return nil
}
