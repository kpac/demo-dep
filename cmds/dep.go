package cmds

import (
	"context"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/dep/gps"
	"github.com/golang/dep/gps/pkgtree"
	"github.com/golang/glog"
	typ "github.com/kubepack/pack/type"
	"github.com/spf13/cobra"
)

var (
	patchDirs      []string
	patchFiles     map[string]string
	imports        []string
	packagePatches map[string]string
)

func NewDepCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Pulls dependent app manifests",
		Run: func(cmd *cobra.Command, args []string) {
			err := runDeps(cmd)
			if err != nil {

				log.Fatalln(err)
			}
		},
	}
	return cmd
}

func runDeps(cmd *cobra.Command) error {
	// Assume the current directory is correctly placed on a GOPATH, and that it's the
	// root of the project.
	packagePatches = make(map[string]string)
	logger := log.New(ioutil.Discard, "", 0)
	if glog.V(glog.Level(1)) {
		logger = log.New(os.Stdout, "", 0)
	}
	root, _ := os.Getwd()
	man := filepath.Join(root, typ.ManifestFile)
	byt, err := ioutil.ReadFile(man)
	manStruc := typ.ManifestDefinition{}
	err = yaml.Unmarshal(byt, &manStruc)
	if err != nil {
		return err
	}

	imports = make([]string, len(manStruc.Dependencies))

	for key, value := range manStruc.Dependencies {
		imports[key] = value.Package
	}

	srcprefix := filepath.Join(build.Default.GOPATH, "src") + string(filepath.Separator)
	importroot := filepath.ToSlash(strings.TrimPrefix(root, srcprefix))

	manifestYaml := ManifestYaml{}
	manifestYaml.root = root
	pkgTree := map[string]pkgtree.PackageOrErr{
		"github.com/sdboyer/gps": {
			P: pkgtree.Package{
				// Name:       "github.com/a8uhnf/go_stack",
				// ImportPath: "github.com/packsh/demo-dep",
				Imports: imports,
			},
		},
	}
	params := gps.SolveParameters{
		RootDir:         root,
		TraceLogger:     logger,
		ProjectAnalyzer: NaiveAnalyzer{},
		Manifest:        manifestYaml,
		RootPackageTree: pkgtree.PackageTree{
			ImportRoot: importroot,
			Packages:   pkgTree,
		},
	}
	// Set up a SourceManager. This manages interaction with sources (repositories).
	tempdir, _ := ioutil.TempDir("", "gps-repocache")
	srcManagerConfig := gps.SourceManagerConfig{
		Cachedir:       filepath.Join(tempdir),
		Logger:         logger,
		DisableLocking: true,
	}
	log.Println("Tempdir: ", tempdir)
	sourcemgr, _ := gps.NewSourceManager(srcManagerConfig)
	defer sourcemgr.Release()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Minute)
	defer cancel()
	// Prep and run the solver
	solver, err := gps.Prepare(params, sourcemgr)
	if err != nil {
		return err
	}
	solution, err := solver.Solve(ctx)
	if err != nil {
		return err
	}
	if err == nil {
		// If no failure, blow away the vendor dir and write a new one out,
		// stripping nested vendor directories as we go.
		err = os.RemoveAll(filepath.Join(root, _VendorFolder))
		if err != nil {
			return err
		}
		err = gps.WriteDepTree(filepath.Join(root, _VendorFolder), solution, sourcemgr, true, logger)
		if err != nil {
			return err
		}

		patchFiles = make(map[string]string)
		err = filepath.Walk(filepath.Join(root, _VendorFolder), findPatchFolder)
		if err != nil {
			return err
		}
	}
	return nil
}

func findPatchFolder(path string, fileInfo os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if strings.HasSuffix(path, PatchFolder) {
		patchDirs = append(patchDirs, path)
	}

	if !strings.Contains(path, PatchFolder) {
		return nil
	}
	if fileInfo.IsDir() {
		return nil
	}

	splitVendor := strings.Split(path, _VendorFolder)
	forkDir := strings.TrimPrefix(strings.Split(splitVendor[1], PatchFolder)[0], "/")
	fmt.Println("Fork Repo---------------", forkDir)
	patchFilePath := strings.TrimPrefix(strings.Split(path, PatchFolder)[1], "/")
	srcDir := filepath.Join(strings.Split(path, _VendorFolder)[0], _VendorFolder, patchFilePath)
	fmt.Println("patchFilePath------", patchFilePath)
	fmt.Println("srcDir-------------", srcDir)
	fmt.Println("filepath-----------", strings.TrimPrefix(patchFilePath, "/"))
	fmt.Println("filepath------XXXXX", strings.Split(strings.Split(path, _VendorFolder)[1], PatchFolder)[0])
	if val, ok := packagePatches[patchFilePath]; ok {
		fmt.Println("hello ---------------------", val)
		fmt.Println("hello ---------------------", strings.TrimPrefix(forkDir, "/"))
		if val == strings.TrimSuffix(strings.TrimPrefix(forkDir, "/"), "/") {
			fmt.Println("hello ---------------------", ok)
		}
	}
	if _, ok := patchFiles[patchFilePath]; ok {
		return fmt.Errorf("Multiple patch on same file: %s", patchFilePath)
	}
	if _, err := os.Stat(srcDir); err == nil {
		patchFiles[patchFilePath] = path
		srcYaml, err := ioutil.ReadFile(srcDir)
		if err != nil {
			return err
		}

		patchYaml, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		mergedYaml, err := CompileWithPatch(srcYaml, patchYaml)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(srcDir, mergedYaml, 0755)
		if err != nil {
			return err
		}
	}
	return err
}

func findImportInManifest(repo string) bool {
	for _, val := range imports {
		if repo == val {
			return true
		}
	}
	return false
}

type NaiveAnalyzer struct {
}

// DeriveManifestAndLock is called when the solver needs manifest/lock data
// for a particular dependency project (identified by the gps.ProjectRoot
// parameter) at a particular version. That version will be checked out in a
// directory rooted at path.
func (a NaiveAnalyzer) DeriveManifestAndLock(path string, n gps.ProjectRoot) (gps.Manifest, gps.Lock, error) {
	// this check should be unnecessary, but keeping it for now as a canary
	if _, err := os.Lstat(path); err != nil {
		return nil, nil, fmt.Errorf("No directory exists at %s; cannot produce ProjectInfo", path)
	}

	m, l, err := a.lookForManifest(path)
	if err == nil {
		// TODO verify project name is same as what SourceManager passed in?
		return m, l, nil
	} else {
		return nil, nil, err
	}
}

// Reports the name and version of the analyzer. This is used internally as part
// of gps' hashing memoization scheme.
func (a NaiveAnalyzer) Info() gps.ProjectAnalyzerInfo {
	return gps.ProjectAnalyzerInfo{
		Name:    "kubernetes-dependency-mngr",
		Version: 1,
	}
}

type ManifestYaml struct {
	root string
}

func (a ManifestYaml) IgnoredPackages() *pkgtree.IgnoredRuleset {
	return nil
}

func (a ManifestYaml) RequiredPackages() map[string]bool {
	return nil
}

func (a ManifestYaml) Overrides() gps.ProjectConstraints {
	ovrr := gps.ProjectConstraints{}

	mpath := filepath.Join(a.root, typ.ManifestFile)
	byt, err := ioutil.ReadFile(mpath)
	manStruc := typ.ManifestDefinition{}
	err = yaml.Unmarshal(byt, &manStruc)
	if err != nil {
		log.Fatalln("Error Occuered-----", err)
	}

	for _, value := range manStruc.Dependencies {
		properties := gps.ProjectProperties{}
		if value.Repo != "" {
			properties.Source = value.Repo
		} else {
			properties.Source = value.Package
		}
		if value.Branch != "" {
			properties.Constraint = gps.NewBranch(value.Branch)
		} else if value.Version != "" {
			properties.Constraint = gps.Revision(value.Version)
		}
		ovrr[gps.ProjectRoot(value.Package)] = properties
	}
	return ovrr
}

func (a ManifestYaml) DependencyConstraints() gps.ProjectConstraints {
	projectConstraints := make(gps.ProjectConstraints)

	man := filepath.Join(a.root, typ.ManifestFile)
	byt, err := ioutil.ReadFile(man)
	manStruc := typ.ManifestDefinition{}
	err = yaml.Unmarshal(byt, &manStruc)
	if err != nil {
		log.Fatalln("Error Occuered-----", err)
	}

	for _, value := range manStruc.Dependencies {
		fmt.Println("hello patch erray***********", value.Patch)
		properties := gps.ProjectProperties{}
		if value.Repo != "" {
			properties.Source = value.Repo
		} else {
			properties.Source = value.Package
		}
		if value.Branch != "" {
			properties.Constraint = gps.NewBranch(value.Branch)
		} else if value.Version != "" {
			properties.Constraint = gps.Revision(value.Version)
		}
		mapPatches(value.Package, value.Patch)
		projectConstraints[gps.ProjectRoot(value.Package)] = properties
	}
	return projectConstraints
}

func mapPatches(repo string, patches []string) {
	for _, val := range patches {
		packagePatches[val] = repo
	}
}

func (a ManifestYaml) TestDependencyConstraints() gps.ProjectConstraints {
	return nil
}

type InternalManifest struct {
	root string
}

func (a InternalManifest) DependencyConstraints() gps.ProjectConstraints {
	projectConstraints := make(gps.ProjectConstraints)

	man := filepath.Join(a.root, typ.ManifestFile)
	byt, err := ioutil.ReadFile(man)
	manStruc := typ.ManifestDefinition{}
	err = yaml.Unmarshal(byt, &manStruc)
	if err != nil {
		log.Fatalln("Error Occuered-----", err)
	}

	for _, value := range manStruc.Dependencies {
		properties := gps.ProjectProperties{}
		if value.Repo != "" {
			properties.Source = value.Repo
		} else {
			properties.Source = value.Package
		}
		if value.Branch != "" {
			properties.Constraint = gps.NewBranch(value.Branch)
		} else if value.Version != "" {
			properties.Constraint = gps.Revision(value.Version)
		}
		projectConstraints[gps.ProjectRoot(value.Package)] = properties
	}
	return projectConstraints
}

type InternalLock struct {
	root string
}

func (a InternalLock) Projects() []gps.LockedProject {
	return nil
}

func (a InternalLock) InputsDigest() []byte {
	return nil
}

func (a NaiveAnalyzer) lookForManifest(root string) (gps.Manifest, gps.Lock, error) {
	mpath := filepath.Join(root, typ.ManifestFile)
	if _, err := os.Lstat(mpath); err != nil {
		return nil, nil, err
	}
	man := &InternalManifest{}
	man.root = root
	lck := &InternalLock{}
	lck.root = root
	return man, lck, nil
}
