package docker

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

const buildahExe = "buildah"

type (
	// Login defines Docker login parameters.
	Login struct {
		Registry string // Docker registry address
		Username string // Docker registry username
		Password string // Docker registry password
		Email    string // Docker registry email
		Config   string // Docker Auth Config
	}

	// Build defines Docker build parameters.
	Build struct {
		Remote      string   // Git remote URL
		Name        string   // Docker build using default named tag
		Dockerfile  string   // Docker build Dockerfile
		Context     string   // Docker build context
		Tags        []string // Docker build tags
		Args        []string // Docker build args
		ArgsEnv     []string // Docker build args from env
		Target      string   // Docker build target
		Squash      bool     // Docker build squash
		Pull        bool     // Docker build pull
		CacheFrom   []string // Docker build cache-from. It is a NOOP in buildah
		Compress    bool     // Docker build compress
		Repo        string   // Docker build repository
		LabelSchema []string // label-schema Label map
		AutoLabel   bool     // auto-label bool
		Labels      []string // Label map
		Link        string   // Git repo link
		NoCache     bool     // Docker build no-cache
		AddHost     []string // Docker build add-host
		Quiet       bool     // Docker build quiet
		S3CacheDir  string
		S3Bucket    string
		S3Endpoint  string
		S3Region    string
		S3Key       string
		S3Secret    string
		S3UseSSL    bool
		Layers      bool
	}

	// Plugin defines the Docker plugin parameters.
	Plugin struct {
		Login        Login  // Docker login configuration
		Build        Build  // Docker build configuration
		Dryrun       bool   // Docker push is skipped
		Cleanup      bool   // Docker purge is enabled
		PushOnly     bool   // Push only mode, skips build process
		SourceTarPath string // Path to Docker image tar file to load and push
		TarPath      string // Path to save Docker image as tar file
		OCIArchive   bool   // Use OCI archive format (true=oci-archive, false=docker-archive)
	}
)

// Exec executes the plugin step
func (p Plugin) Exec() error {
	// Create Auth Config File
	if p.Login.Config != "" {
		user, err := user.Current()
		if err != nil {
			return fmt.Errorf("Error getting the current user: %s", err)
		}
		root := fmt.Sprintf("/var/tmp/%s/containers/containers/", user.Uid)
		if err := os.MkdirAll(root, 0777); err != nil {
			return fmt.Errorf("Error writing runtime dir: %s", err)
		}

		path := filepath.Join(root, "auth.json")
		if err := ioutil.WriteFile(path, []byte(p.Login.Config), 0600); err != nil {
			return fmt.Errorf("Error writing auth.json: %s", err)
		}

		fmt.Printf("Config written to %s\n", path)
	}

	// login to the Docker registry
	if p.Login.Password != "" {
		cmd := commandLogin(p.Login)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("Error authenticating: %s", err)
		}
	}

	switch {
	case p.Login.Password != "":
		fmt.Println("Detected registry credentials")
	case p.Login.Config != "":
		fmt.Println("Detected registry credentials file")
	default:
		fmt.Println("Registry credentials or Docker config not provided. Guest mode enabled.")
	}

	// Check if we're in push-only mode
	if p.PushOnly {
		return p.pushOnly()
	}

	// add proxy build args
	addProxyBuildArgs(&p.Build)

	var cmds []*exec.Cmd
	cmds = append(cmds, commandVersion()) // docker version
	cmds = append(cmds, commandInfo())    // docker info

	// pre-pull cache images
	for _, img := range p.Build.CacheFrom {
		cmds = append(cmds, commandPull(img))
	}

	cmds = append(cmds, commandBuild(p.Build)) // docker build

	for _, tag := range p.Build.Tags {
		cmds = append(cmds, commandTag(p.Build, tag)) // docker tag

		if !p.Dryrun {
			cmds = append(cmds, commandPush(p.Build, tag)) // docker push
		}
	}

	// If TarPath is specified and Dryrun is enabled, save the image to a tar file
	if p.TarPath != "" && p.Dryrun && len(p.Build.Tags) > 0 {
		// Create parent directories if they don't exist
		dir := filepath.Dir(p.TarPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directories for tar file: %s", err)
		}

		imageToSave := fmt.Sprintf("%s:%s", p.Build.Repo, p.Build.Tags[0])
		fmt.Println("Saving image to tar:", p.TarPath)
		cmds = append(cmds, commandSaveTar(imageToSave, p.TarPath, p.OCIArchive))
	}

	if p.Cleanup {
		cmds = append(cmds, commandRmi(p.Build.Name)) // buildah rmi
	}

	// execute all commands in batch mode.
	for _, cmd := range cmds {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		trace(cmd)

		err := cmd.Run()
		if err != nil && isCommandPull(cmd.Args) {
			fmt.Printf("Could not pull cache-from image %s. Ignoring...\n", cmd.Args[2])
		} else if err != nil && isCommandPrune(cmd.Args) {
			fmt.Printf("Could not prune system containers. Ignoring...\n")
		} else if err != nil && isCommandRmi(cmd.Args) {
			fmt.Printf("Could not remove image %s. Ignoring...\n", cmd.Args[2])
		} else if err != nil {
			return err
		}
	}

	return nil
}

// helper function to create the docker login command.
func commandLogin(login Login) *exec.Cmd {
	if login.Email != "" {
		return commandLoginEmail(login)
	}
	return exec.Command(
		buildahExe, "login",
		"-u", login.Username,
		"-p", login.Password,
		login.Registry,
	)
}

// helper to check if args match "docker pull <image>"
func isCommandPull(args []string) bool {
	return len(args) > 2 && args[1] == "pull"
}

func commandPull(repo string) *exec.Cmd {
	return exec.Command(buildahExe, "pull", repo)
}

func commandLoginEmail(login Login) *exec.Cmd {
	return exec.Command(
		buildahExe, "login",
		"-u", login.Username,
		"-p", login.Password,
		"-e", login.Email,
		login.Registry,
	)
}

// helper function to create the docker info command.
func commandVersion() *exec.Cmd {
	return exec.Command(buildahExe, "--storage-driver", "vfs", "version")
}

// helper function to create the docker info command.
func commandInfo() *exec.Cmd {
	return exec.Command(buildahExe, "--storage-driver", "vfs", "info")
}

// helper function to create the docker build command.
func commandBuild(build Build) *exec.Cmd {
	args := []string{
		"bud",
		"--storage-driver", "vfs",
		"-f", build.Dockerfile,
	}

	if build.Squash {
		args = append(args, "--squash")
	}
	if build.Compress {
		args = append(args, "--compress")
	}
	if build.Pull {
		args = append(args, "--pull=true")
	}
	if build.NoCache {
		args = append(args, "--no-cache")
	}
	for _, arg := range build.CacheFrom {
		args = append(args, "--cache-from", arg)
	}
	for _, arg := range build.ArgsEnv {
		addProxyValue(&build, arg)
	}
	for _, arg := range build.Args {
		args = append(args, "--build-arg", arg)
	}
	for _, host := range build.AddHost {
		args = append(args, "--add-host", host)
	}
	if build.Target != "" {
		args = append(args, "--target", build.Target)
	}
	if build.Quiet {
		args = append(args, "--quiet")
	}
	if build.Layers {
		args = append(args, "--layers=true")
		if build.S3CacheDir != "" {
			args = append(args, "--s3-local-cache-dir", build.S3CacheDir)
			if build.S3Bucket != "" {
				args = append(args, "--s3-bucket", build.S3Bucket)
			}
			if build.S3Endpoint != "" {
				args = append(args, "--s3-endpoint", build.S3Endpoint)
			}
			if build.S3Region != "" {
				args = append(args, "--s3-region", build.S3Region)
			}
			if build.S3Key != "" {
				args = append(args, "--s3-key", build.S3Key)
			}
			if build.S3Secret != "" {
				args = append(args, "--s3-secret", build.S3Secret)
			}
			if build.S3UseSSL {
				args = append(args, "--s3-use-ssl=true")
			}
		}
	}

	if build.AutoLabel {
		labelSchema := []string{
			fmt.Sprintf("created=%s", time.Now().Format(time.RFC3339)),
			fmt.Sprintf("revision=%s", build.Name),
			fmt.Sprintf("source=%s", build.Remote),
			fmt.Sprintf("url=%s", build.Link),
		}
		labelPrefix := "org.opencontainers.image"

		if len(build.LabelSchema) > 0 {
			labelSchema = append(labelSchema, build.LabelSchema...)
		}

		for _, label := range labelSchema {
			args = append(args, "--label", fmt.Sprintf("%s.%s", labelPrefix, label))
		}
	}

	if len(build.Labels) > 0 {
		for _, label := range build.Labels {
			args = append(args, "--label", label)
		}
	}

	args = append(args, "-t", build.Name)
	args = append(args, build.Context)
	return exec.Command(buildahExe, args...)
}

// helper function to add proxy values from the environment
func addProxyBuildArgs(build *Build) {
	addProxyValue(build, "http_proxy")
	addProxyValue(build, "https_proxy")
	addProxyValue(build, "no_proxy")
}

// helper function to add the upper and lower case version of a proxy value.
func addProxyValue(build *Build, key string) {
	value := getProxyValue(key)

	if len(value) > 0 && !hasProxyBuildArg(build, key) {
		build.Args = append(build.Args, fmt.Sprintf("%s=%s", key, value))
		build.Args = append(build.Args, fmt.Sprintf("%s=%s", strings.ToUpper(key), value))
	}
}

// helper function to get a proxy value from the environment.
//
// assumes that the upper and lower case versions of are the same.
func getProxyValue(key string) string {
	value := os.Getenv(key)

	if len(value) > 0 {
		return value
	}

	return os.Getenv(strings.ToUpper(key))
}

// helper function that looks to see if a proxy value was set in the build args.
func hasProxyBuildArg(build *Build, key string) bool {
	keyUpper := strings.ToUpper(key)

	for _, s := range build.Args {
		if strings.HasPrefix(s, key) || strings.HasPrefix(s, keyUpper) {
			return true
		}
	}

	return false
}

// helper function to create the docker tag command.
func commandTag(build Build, tag string) *exec.Cmd {
	var (
		source = build.Name
		target = fmt.Sprintf("%s:%s", build.Repo, tag)
	)
	return exec.Command(
		buildahExe, "tag", "--storage-driver", "vfs", source, target,
	)
}

// helper function to create the docker push command.
func commandPush(build Build, tag string) *exec.Cmd {
	target := fmt.Sprintf("%s:%s", build.Repo, tag)
	return exec.Command(buildahExe, "push", "--storage-driver", "vfs", target)
}

// helper to check if args match "docker prune"
func isCommandPrune(args []string) bool {
	return len(args) > 3 && args[2] == "prune"
}

// helper to check if args match "docker rmi"
func isCommandRmi(args []string) bool {
	return len(args) > 2 && args[1] == "rmi"
}

func commandRmi(tag string) *exec.Cmd {
	return exec.Command(buildahExe, "--storage-driver", "vfs", "rmi", tag)
}

// trace writes each command to stdout with the command wrapped in an xml
// tag so that it can be extracted and displayed in the logs.
func trace(cmd *exec.Cmd) {
	fmt.Fprintf(os.Stdout, "+ %s\n", strings.Join(cmd.Args, " "))
}

// pushOnly handles pushing images without building them
func (p Plugin) pushOnly() error {
	// If source tar path is provided, load the image first
	if p.SourceTarPath != "" {
		fileInfo, err := os.Stat(p.SourceTarPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("source image tar file %s does not exist", p.SourceTarPath)
			}
			return fmt.Errorf("failed to access source image tar file: %s", err)
		}

		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("source image tar %s is not a regular file", p.SourceTarPath)
		}

		fmt.Println("Loading image from tar:", p.SourceTarPath)
		loadCmd := commandLoadTar(p.SourceTarPath, p.OCIArchive)
		loadCmd.Stdout = os.Stdout
		loadCmd.Stderr = os.Stderr
		trace(loadCmd)
		if err := loadCmd.Run(); err != nil {
			return fmt.Errorf("failed to load image from tar: %s", err)
		}
	}

	// Check for required tags
	if len(p.Build.Tags) == 0 {
		return fmt.Errorf("no tags specified for push")
	}

	// After loading the image from tar, find the loaded image
	loadedImage, err := findLoadedImage(p.Build.Repo)
	if err != nil {
		return fmt.Errorf("failed to find loaded image: %w", err)
	}
	
	// Create a map to track which images have been tagged for pushing
	taggedImages := make(map[string]bool)
	
	// First check if any of our target tags already exist
	for _, tag := range p.Build.Tags {
		fullImage := fmt.Sprintf("%s:%s", p.Build.Repo, tag)
		if imageExists(fullImage) {
			taggedImages[fullImage] = true
			continue
		}
	}
	
	// If the loaded image exists and we have tags to create
	if loadedImage != "" {
		// Create additional tags for any tags that don't exist
		for _, tag := range p.Build.Tags {
			fullImage := fmt.Sprintf("%s:%s", p.Build.Repo, tag)
			if !taggedImages[fullImage] {
				// Tag the loaded image with this tag
				tagCmd := commandTag(p.Build, tag)
				tagCmd.Stdout = os.Stdout
				tagCmd.Stderr = os.Stderr
				trace(tagCmd)
				if err := tagCmd.Run(); err == nil {
					taggedImages[fullImage] = true
					fmt.Printf("Created tag: %s\n", fullImage)
				}
			}
		}
	}
	
	// If no images were tagged or found, we can't proceed
	if len(taggedImages) == 0 {
		return fmt.Errorf("no images found or tagged for repository %s, cannot push", p.Build.Repo)
	}

	var cmds []*exec.Cmd
	
	// Push all tagged images
	for tag := range taggedImages {
		// Extract tag from the full image name
		_, tagOnly, found := strings.Cut(tag, ":")
		if !found {
			continue
		}
		
		// Push the image if not in dry-run mode
		if !p.Dryrun {
			cmds = append(cmds, commandPush(p.Build, tagOnly))
		}
	}

	// Tar saving functionality is only available in the regular execution mode with dry-run enabled

	// Execute all commands
	for _, cmd := range cmds {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		trace(cmd)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed: %s", err)
		}
	}

	return nil
}

// getArchiveFormat returns the appropriate archive format prefix based on the OCIArchive flag
func getArchiveFormat(useOCIArchive bool) string {
	if useOCIArchive {
		return "oci-archive:"
	}
	return "docker-archive:"
}

// commandLoadTar creates a command to load an image from a tar file
func commandLoadTar(tarPath string, useOCIArchive bool) *exec.Cmd {
	archiveFormat := getArchiveFormat(useOCIArchive)
	return exec.Command(buildahExe, "--storage-driver", "vfs", "pull", archiveFormat+tarPath)
}

// commandImageExists creates a command to check if an image exists
func commandImageExists(image string) *exec.Cmd {
	return exec.Command(buildahExe, "inspect", "--storage-driver", "vfs", "--type", "image", image)
}

// commandSaveTar creates a command to save an image to a tar file
func commandSaveTar(image string, tarPath string, useOCIArchive bool) *exec.Cmd {
	archiveFormat := getArchiveFormat(useOCIArchive)
	return exec.Command(buildahExe, "push", "--storage-driver", "vfs", image, archiveFormat+tarPath)
}

// findLoadedImage finds the image that was loaded from the tar file by listing all images
// and matching them against the repository name
func findLoadedImage(repo string) (string, error) {
	// List all images in storage
	listCmd := exec.Command(buildahExe, "--storage-driver", "vfs", "images", "--format", "{{.Repository}}:{{.Tag}}")
	var output bytes.Buffer
	listCmd.Stdout = &output
	listCmd.Stderr = os.Stderr
	trace(listCmd)
	if err := listCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to list images: %w", err)
	}

	// Process the output to find a matching image
	images := strings.Split(strings.TrimSpace(output.String()), "\n")
	for _, img := range images {
		if strings.HasPrefix(img, repo) {
			return img, nil
		}
	}

	return "", fmt.Errorf("no matching image found for repository %s", repo)
}

// imageExists checks if an image exists in the buildah storage
func imageExists(image string) bool {
	existsCmd := commandImageExists(image)
	existsCmd.Stdout = nil // suppress output, we only care about the exit code
	existsCmd.Stderr = os.Stderr
	trace(existsCmd)
	return existsCmd.Run() == nil
}
