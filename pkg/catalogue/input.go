package catalogue

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/nuclei/v2/pkg/quarks"
	"github.com/projectdiscovery/nuclei/v2/pkg/quarks/templates"
	"github.com/projectdiscovery/nuclei/v2/pkg/quarks/workflows"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v2"
)

// Input is the input read from the template file defining all components
// of the template file and also specify whether loaded input is a
// workflow or a template.
type Input struct {
	// ID is the unique id for the template
	ID string `yaml:"id"`
	// Info contains information about the template
	Info quarks.Info `yaml:"info"`

	// Embed the template structure in the input itself.
	templates.Template `yaml:",inline"`

	// Embed the workflow structure in the input itself.
	workflows.Workflow `yaml:",inline"`
}

// CompiledInput is the compiled version of a input
type CompiledInput struct {
	// Type is the type of the input provided
	Type Type

	*templates.CompiledTemplate
	*workflows.CompiledWorkflow
}

// ReadInput reads a template input from disk returning
// a validated version of the template.
func ReadInput(path string) (*Input, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	validationInput := new(interface{})
	if err = unmarshalForValidation(data, validationInput); err != nil {
		return nil, err
	}

	result, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewGoLoader(validationInput))
	if err != nil {
		return nil, err
	}

	// Show number of errors and also the first error
	if !result.Valid() {
		errs := result.Errors()
		return nil, errors.Errorf("%d errors in template: %s, skipping", len(errs), errs[0])
	}

	input := &Input{}
	if err := yaml.Unmarshal(data, input); err != nil {
		return nil, err
	}
	return input, nil
}

// Compile returns the compiled version of the input
func (i *Input) Compile(catalog *Catalogue, path string) (*CompiledInput, error) {
	Type, ok := i.getType()
	if !ok {
		return nil, errors.New("invalid template/workflow supplied")
	}

	compiled := &CompiledInput{
		Type: Type,
	}
	if Type == TemplateInputType {
		compiledTemplate, err := i.Template.Compile(templates.CompileOptions{
			ID:       i.ID,
			Info:     i.Info,
			Path:     path,
			Resolver: catalog,
		})
		if err != nil {
			return nil, errors.Wrap(err, "could not compile template")
		}
		compiled.CompiledTemplate = compiledTemplate
	}
	if Type == WorkflowInputType {
		compiledWorkflow, err := i.Workflow.Compile(workflows.CompileOptions{
			ID:       i.ID,
			Info:     i.Info,
			Path:     path,
			Resolver: catalog,
			Compiler: catalog,
		})
		if err != nil {
			return nil, errors.Wrap(err, "could not compile workflow")
		}
		compiled.CompiledWorkflow = compiledWorkflow
	}
	return compiled, nil
}

// Type is the type of the input provided
type Type int

// Input types we can process
const (
	TemplateInputType Type = iota
	WorkflowInputType
)

// getType returns the type of input provided based on various attributes
func (i *Input) getType() (Type, bool) {
	if len(i.DNS) > 0 || len(i.HTTP) > 0 || len(i.HTTPRequests) > 0 {
		return TemplateInputType, true
	}
	if len(i.Logic) > 0 {
		return WorkflowInputType, true
	}
	return -1, false
}