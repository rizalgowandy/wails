// +build linux

package packagemanager

import (
	"sort"
	"strings"

	"github.com/leaanthony/wailsv2/v2/internal/shell"
)

// PackageManager is a common interface across all package managers
type PackageManager interface {
	Name() string
	Packages() packagemap
	PackageInstalled(*Package) (bool, error)
	PackageAvailable(*Package) (bool, error)
	InstallCommand(*Package) string
}

// Package contains information about a system package
type Package struct {
	Name           string
	Version        string
	InstallCommand map[string]string
	SystemPackage  bool
	Library        bool
	Optional       bool
}

// A list of package manager commands
var pmcommands = []string{
	"eopkg",
	"apt",
	"yum",
	"pacman",
	"emerge",
	"zypper",
}

type packagemap = map[string][]*Package

// Find will attempt to find the system package manager
func Find(osid string) PackageManager {

	// Loop over pmcommands
	for _, pmname := range pmcommands {
		if shell.CommandExists(pmname) {
			return newPackageManager(pmname, osid)
		}
	}
	return nil
}

func newPackageManager(pmname string, osid string) PackageManager {
	switch pmname {
	case "eopkg":
		return NewEopkg(osid)
	case "apt":
		return NewApt(osid)
	case "yum":
		return NewYum(osid)
	case "pacman":
		return NewPacman(osid)
	case "emerge":
		return NewEmerge(osid)
	case "zypper":
		return NewZypper(osid)
	}
	return nil
}

// Dependancy represents a system package that we require
type Dependancy struct {
	Name           string
	PackageName    string
	Installed      bool
	InstallCommand string
	Version        string
	Optional       bool
	External       bool
}

// DependencyList is a list of Dependency instances
type DependencyList []*Dependancy

// InstallAllRequiredCommand returns the command you need to use to install all required dependencies
func (d DependencyList) InstallAllRequiredCommand() string {

	result := ""
	for _, dependency := range d {
		if dependency.PackageName != "" {
			if !dependency.Installed && !dependency.Optional {
				if result == "" {
					result = dependency.InstallCommand
				} else {
					result += " " + dependency.PackageName
				}
			}
		}
	}

	return result
}

// InstallAllOptionalCommand returns the command you need to use to install all optional dependencies
func (d DependencyList) InstallAllOptionalCommand() string {

	result := ""
	for _, dependency := range d {
		if dependency.PackageName != "" {
			if !dependency.Installed && dependency.Optional {
				if result == "" {
					result = dependency.InstallCommand
				} else {
					result += " " + dependency.PackageName
				}
			}
		}
	}

	return result
}

// Dependancies scans the system for required dependancies
// Returns a list of dependancies search for, whether they were found
// and whether they were installed
func Dependancies(p PackageManager) (DependencyList, error) {

	var dependencies DependencyList

	for name, packages := range p.Packages() {
		dependency := &Dependancy{Name: name}
		for _, pkg := range packages {
			dependency.Optional = pkg.Optional
			dependency.External = !pkg.SystemPackage
			dependency.InstallCommand = p.InstallCommand(pkg)
			packageavailable, err := p.PackageAvailable(pkg)
			if err != nil {
				return nil, err
			}
			if packageavailable {
				dependency.Version = pkg.Version
				dependency.PackageName = pkg.Name
				installed, err := p.PackageInstalled(pkg)
				if err != nil {
					return nil, err
				}
				if installed {
					dependency.Installed = true
					dependency.Version = pkg.Version
					if !pkg.Library {
						dependency.Version = AppVersion(name)
					}
				} else {
					dependency.InstallCommand = p.InstallCommand(pkg)
				}
				break
			}
		}
		dependencies = append(dependencies, dependency)
	}

	// Sort dependencies
	sort.Slice(dependencies, func(i, j int) bool {
		return dependencies[i].Name < dependencies[j].Name
	})

	return dependencies, nil
}

// AppVersion returns the version for application related to the given package
func AppVersion(name string) string {

	if name == "gcc" {
		return gccVersion()
	}

	if name == "pkg-config" {
		return pkgConfigVersion()
	}

	if name == "npm" {
		return npmVersion()
	}

	if name == "docker" {
		return dockerVersion()
	}

	return ""

}

func gccVersion() string {

	var version string
	var err error

	// Try "-dumpfullversion"
	version, _, err = shell.RunCommand(".", "gcc", "-dumpfullversion")
	if err != nil {

		// Try -dumpversion
		// We ignore the error as this function is not for testing whether the
		// application exists, only that we can get the version number
		dumpversion, _, err := shell.RunCommand(".", "gcc", "-dumpversion")
		if err == nil {
			version = dumpversion
		}
	}
	return strings.TrimSpace(version)
}

func pkgConfigVersion() string {
	version, _, _ := shell.RunCommand(".", "pkg-config", "--version")
	return strings.TrimSpace(version)
}

func npmVersion() string {
	version, _, _ := shell.RunCommand(".", "npm", "--version")
	return strings.TrimSpace(version)
}

func dockerVersion() string {
	version, _, _ := shell.RunCommand(".", "docker", "--version")
	version = strings.TrimPrefix(version, "Docker version ")
	version = strings.ReplaceAll(version, ", build ", " (")
	version = strings.TrimSpace(version) + ")"
	return version
}
