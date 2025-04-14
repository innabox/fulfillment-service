/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/spf13/pflag"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

// GrpcRulesAuthzType is the name of the rules authentication function.
const GrpcRulesAuthzType = "rules"

// GrpcRulesAuthzFuncBuilder contains the data and logic needed to create an authorization function that makes decisions
// based on a set of rules read from a file.
type GrpcRulesAuthzFuncBuilder struct {
	logger    *slog.Logger
	ruleFiles []string
}

type grpcRulesAuthzFunc struct {
	logger *slog.Logger
	celEnv *cel.Env
	rules  []grpcAuthzRule
}

type grpcAuthzRuleAction int

const (
	grpcAuthzRuleActionAllow grpcAuthzRuleAction = iota
	grpcAuthzRuleActionDeny
)

type grpcAuthzRule struct {
	name      string
	condition cel.Program
	action    grpcAuthzRuleAction
}

// NewGrpcRulesAuthzFunc creates a builder that can then be configured and used to create a new authorization function.
func NewGrpcRulesAuthzFunc() *GrpcRulesAuthzFuncBuilder {
	return &GrpcRulesAuthzFuncBuilder{}
}

// SetLogger sets the logger that the function will use to write messages to the log. This is mandatory.
func (b *GrpcRulesAuthzFuncBuilder) SetLogger(value *slog.Logger) *GrpcRulesAuthzFuncBuilder {
	b.logger = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the function. This is optional.
func (b *GrpcRulesAuthzFuncBuilder) SetFlags(flags *pflag.FlagSet) *GrpcRulesAuthzFuncBuilder {
	if flags == nil {
		return b
	}

	if flags.Changed(grpcRulesAuthzFileFlagName) {
		values, err := flags.GetStringArray(grpcRulesAuthzFileFlagName)
		if err == nil {
			for _, value := range values {
				b.AddRulesFile(value)
			}
		}
	}

	return b
}

// AddRulesFile adds a file that contains authorization rules. This should be a YAML file with the following format:
//
//   - condition: subjetc.name == 'myuser'
//     action: allow
//
//   - condition: subject.claims['group'] != 'admin' && method.startsWith('/private.v1.')
//     action: deny
//
// Each rule has a condition and an action. The condition is a CEL boolean expression that decides if the rule matches
// or not. The action can be `allow` or `deny`. The first rule that matches determines if access is allowed or denied.
//
// The CEL expressions can use the builtin variables `subject` and `method`. The `subject` is an object that has `name`
// and `claims` fields, see the Subject type for details about those fields. The `method` is the full name of the gRPC
// method, for example `/fulfillment.v1.Clusters/List`.
func (b *GrpcRulesAuthzFuncBuilder) AddRulesFile(value string) *GrpcRulesAuthzFuncBuilder {
	if value != "" {
		b.ruleFiles = append(b.ruleFiles, value)
	}
	return b
}

// Build uses the data stored in the builder to create a new authorization function.
func (b *GrpcRulesAuthzFuncBuilder) Build() (result GrpcAuthzFunc, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create the CEL environment:
	celRegistry, err := types.NewRegistry()
	if err != nil {
		err = fmt.Errorf("failed to create type registry: %w", err)
		return
	}
	celProvider := &celTypeProvider{
		registry: celRegistry,
	}
	celEnv, err := cel.NewEnv(
		cel.CustomTypeProvider(celProvider),
		cel.Variable(grpcRulesAuthzSubjecVar, celSubjectType),
		cel.Variable(grpcRulesAuthzMethodVar, cel.StringType),
		cel.Variable(grpcRulesAuthzNowVar, cel.TimestampType),
	)
	if err != nil {
		err = fmt.Errorf("failed to create CEL environment: %w", err)
		return
	}

	// Load the rules:
	var rules []grpcAuthzRule
	for _, file := range b.ruleFiles {
		var fileRules []grpcAuthzRule
		fileRules, err = b.loadRules(celEnv, file)
		if err != nil {
			return
		}
		rules = append(rules, fileRules...)
	}

	// Create and populate the object:
	object := &grpcRulesAuthzFunc{
		logger: b.logger,
		celEnv: celEnv,
		rules:  rules,
	}
	result = object.call

	return
}

// grpcAuthzRule is the type used to read a single rule from a YAML document.
type grpcAuthzRuleData struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"`
	Action    string `yaml:"action"`
}

func (b *GrpcRulesAuthzFuncBuilder) loadRules(celEnv *cel.Env, file string) (result []grpcAuthzRule, err error) {
	rulesYaml, err := os.ReadFile(file)
	if err != nil {
		err = fmt.Errorf("failed to load rules from file '%s': %w", file, err)
		return
	}
	var rulesData []grpcAuthzRuleData
	err = yaml.Unmarshal(rulesYaml, &rulesData)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal rules from file '%s': %w", file, err)
		return
	}
	var rules []grpcAuthzRule
	for i, ruleData := range rulesData {
		if ruleData.Name == "" {
			ruleData.Name = fmt.Sprintf("Rule %d", i)
		}
		var rule grpcAuthzRule
		rule, err = b.compileRule(celEnv, ruleData)
		if err != nil {
			err = fmt.Errorf("failed to compile rule '%s' from file '%s': %w", rule.name, file, err)
			return
		}
		rules = append(rules, rule)
	}
	result = rules
	return
}

func (b *GrpcRulesAuthzFuncBuilder) compileRule(celEnv *cel.Env, ruleData grpcAuthzRuleData) (result grpcAuthzRule,
	err error) {
	ast, issues := celEnv.Compile(ruleData.Condition)
	err = issues.Err()
	if err != nil {
		err = fmt.Errorf("failed to compile condition for rule '%s': %w", ruleData.Name, err)
		return
	}
	prg, err := celEnv.Program(ast)
	if err != nil {
		err = fmt.Errorf("failed to create program for condition of rule '%s': %w", ruleData.Name, err)
		return
	}
	var action grpcAuthzRuleAction
	switch strings.ToLower(ruleData.Action) {
	case "allow":
		action = grpcAuthzRuleActionAllow
	case "deny":
		action = grpcAuthzRuleActionDeny
	default:
		err = fmt.Errorf("unknown action '%s' for rule '%s': %w", ruleData.Action, ruleData.Name, err)
		return
	}
	result.name = ruleData.Name
	result.condition = prg
	result.action = action
	return
}

func (f *grpcRulesAuthzFunc) call(ctx context.Context, method string) error {
	subject := SubjectFromContext(ctx)
	logger := f.logger.With(
		slog.String("subject", subject.Name),
	)
	for _, rule := range f.rules {
		matches, err := f.evalCondition(rule.condition, subject, method)
		if err != nil {
			logger.ErrorContext(
				ctx,
				"Failed to evaluate condition",
				slog.String("rule", rule.name),
				slog.Any("error", err),
			)
			return grpcRulesAuthzAccessDenied
		}
		if !matches {
			continue
		}
		switch rule.action {
		case grpcAuthzRuleActionAllow:
			logger.DebugContext(
				ctx,
				"Access allowed",
				slog.String("rule", rule.name),
			)
			return nil
		case grpcAuthzRuleActionDeny:
			logger.DebugContext(
				ctx,
				"Access denied",
				slog.String("rule", rule.name),
			)
			return grpcRulesAuthzAccessDenied
		default:
			logger.DebugContext(
				ctx,
				"Unknown action",
				slog.Int("action", int(rule.action)),
			)
			return grpcRulesAuthzAccessDenied
		}
	}
	logger.DebugContext(
		ctx,
		"No matching rule, access denied",
	)
	return grpcRulesAuthzAccessDenied
}

func (f *grpcRulesAuthzFunc) evalCondition(condition cel.Program, subject *Subject, method string) (result bool,
	err error) {
	output, _, err := condition.Eval(map[string]any{
		grpcRulesAuthzSubjecVar: &celSubject{subject},
		grpcRulesAuthzMethodVar: method,
		grpcRulesAuthzNowVar:    time.Now(),
	})
	if err != nil {
		return
	}
	value := output.Value()
	result, ok := value.(bool)
	if !ok {
		err = fmt.Errorf("expected boolean result but got %T", value)
	}
	return
}

// Names of built-in CEL variables:
const (
	grpcRulesAuthzSubjecVar = "subject"
	grpcRulesAuthzMethodVar = "method"
	grpcRulesAuthzNowVar    = "now"
)

var grpcRulesAuthzAccessDenied = grpcstatus.Errorf(grpccodes.PermissionDenied, "Access denied")
