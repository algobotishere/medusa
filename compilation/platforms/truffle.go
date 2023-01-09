package platforms

import (
	"encoding/json"
	"fmt"
	"github.com/trailofbits/medusa/compilation/types"
	"os"
	"os/exec"
	"path/filepath"
)

type TruffleCompilationConfig struct {
	Target         string `json:"target"`
	UseNpx         bool   `json:"useNpx"`
	Command        string `json:"command"`
	BuildDirectory string `json:"buildDirectory"`
}

func NewTruffleCompilationConfig(target string) *TruffleCompilationConfig {
	return &TruffleCompilationConfig{
		Target:         target,
		UseNpx:         true,
		Command:        "",
		BuildDirectory: "",
	}
}

func (s *TruffleCompilationConfig) Platform() string {
	return "truffle"
}

// GetTarget returns the target for compilation
func (t *TruffleCompilationConfig) GetTarget() string {
	return t.Target
}

// SetTarget sets the new target for compilation
func (t *TruffleCompilationConfig) SetTarget(newTarget string) {
	t.Target = newTarget
}

func (s *TruffleCompilationConfig) Compile() ([]types.Compilation, string, error) {
	// Determine the base command to use.
	var baseCommandStr = "truffle"
	if s.Command != "" {
		baseCommandStr = s.Command
	}

	// Execute solc to compile our target.
	var cmd *exec.Cmd
	if s.UseNpx {
		cmd = exec.Command("npx", baseCommandStr, "compile", "--all")
	} else {
		cmd = exec.Command(baseCommandStr, "compile", "--all")
	}
	cmd.Dir = s.Target
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("error while executing truffle:\nOUTPUT:\n%s\nERROR: %s\n", string(out), err.Error())
	}

	// Create a compilation unit out of this.
	compilation := types.NewCompilation()

	// Find all the compiled truffle artifacts
	buildDirectory := s.BuildDirectory
	if buildDirectory == "" {
		buildDirectory = filepath.Join(s.Target, "build", "contracts")
	}
	matches, err := filepath.Glob(filepath.Join(buildDirectory, "*.json"))
	if err != nil {
		return nil, "", err
	}

	// Define our truffle structure to parse
	type TruffleCompiledJson struct {
		ContractName      string `json:"contractName"`
		Abi               any    `json:"abi"`
		Bytecode          string `json:"bytecode"`
		DeployedBytecode  string `json:"deployedBytecode"`
		SourceMap         string `json:"sourceMap"`
		DeployedSourceMap string `json:"deployedSourceMap"`
		Source            string `json:"source"`
		SourcePath        string `json:"sourcePath"`
		Ast               any    `json:"ast"`
	}

	// Loop for each truffle artifact to parse our compilations.
	for i := 0; i < len(matches); i++ {
		// Read the compiled JSON file data
		b, err := os.ReadFile(matches[i])
		if err != nil {
			return nil, "", err
		}

		// Parse the JSON
		var compiledJson TruffleCompiledJson
		err = json.Unmarshal(b, &compiledJson)
		if err != nil {
			return nil, "", err
		}

		// Convert the abi structure to our parsed abi type
		contractAbi, err := types.ParseABIFromInterface(compiledJson.Abi)
		if err != nil {
			continue
		}

		// If we don't have a source for this file, create it.
		if _, ok := compilation.Sources[compiledJson.SourcePath]; !ok {
			compilation.Sources[compiledJson.SourcePath] = types.CompiledSource{
				Ast:       compiledJson.Ast,
				Contracts: make(map[string]types.CompiledContract),
			}
		}

		// Add our contract to the source
		compilation.Sources[compiledJson.SourcePath].Contracts[compiledJson.ContractName] = types.CompiledContract{
			Abi:             *contractAbi,
			InitBytecode:    compiledJson.Bytecode,
			RuntimeBytecode: compiledJson.DeployedBytecode,
			SrcMapsInit:     compiledJson.SourceMap,
			SrcMapsRuntime:  compiledJson.DeployedSourceMap,
			PlaceholderSet:  types.ParseBytecodeForPlaceholders(compiledJson.Bytecode),
		}
	}

	return []types.Compilation{*compilation}, string(out), nil
}
