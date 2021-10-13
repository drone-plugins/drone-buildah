package main

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	docker "github.com/drone-plugins/drone-buildah"
)

var (
	version = "unknown"
)

func main() {
	// Load env-file if it exists first
	if env := os.Getenv("PLUGIN_ENV_FILE"); env != "" {
		godotenv.Load(env)
	}

	app := cli.NewApp()
	app.Name = "buildah plugin"
	app.Usage = "buildah plugin"
	app.Action = run
	app.Version = version
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "dry-run",
			Usage:  "dry run disables docker push",
			EnvVar: "PLUGIN_DRY_RUN",
		},
		cli.StringFlag{
			Name:   "remote.url",
			Usage:  "git remote url",
			EnvVar: "DRONE_REMOTE_URL",
		},
		cli.StringFlag{
			Name:   "commit.sha",
			Usage:  "git commit sha",
			EnvVar: "DRONE_COMMIT_SHA",
			Value:  "00000000",
		},
		cli.StringFlag{
			Name:   "commit.ref",
			Usage:  "git commit ref",
			EnvVar: "DRONE_COMMIT_REF",
		},
		cli.StringFlag{
			Name:   "dockerfile",
			Usage:  "build dockerfile",
			Value:  "Dockerfile",
			EnvVar: "PLUGIN_DOCKERFILE",
		},
		cli.StringFlag{
			Name:   "context",
			Usage:  "build context",
			Value:  ".",
			EnvVar: "PLUGIN_CONTEXT",
		},
		cli.StringSliceFlag{
			Name:     "tags",
			Usage:    "build tags",
			Value:    &cli.StringSlice{"latest"},
			EnvVar:   "PLUGIN_TAG,PLUGIN_TAGS",
			FilePath: ".tags",
		},
		cli.BoolFlag{
			Name:   "tags.auto",
			Usage:  "default build tags",
			EnvVar: "PLUGIN_DEFAULT_TAGS,PLUGIN_AUTO_TAG",
		},
		cli.StringFlag{
			Name:   "tags.suffix",
			Usage:  "default build tags with suffix",
			EnvVar: "PLUGIN_DEFAULT_SUFFIX,PLUGIN_AUTO_TAG_SUFFIX",
		},
		cli.StringSliceFlag{
			Name:   "args",
			Usage:  "build args",
			EnvVar: "PLUGIN_BUILD_ARGS",
		},
		cli.StringSliceFlag{
			Name:   "args-from-env",
			Usage:  "build args",
			EnvVar: "PLUGIN_BUILD_ARGS_FROM_ENV",
		},
		cli.BoolFlag{
			Name:   "quiet",
			Usage:  "quiet docker build",
			EnvVar: "PLUGIN_QUIET",
		},
		cli.StringFlag{
			Name:   "target",
			Usage:  "build target",
			EnvVar: "PLUGIN_TARGET",
		},
		cli.BoolFlag{
			Name:   "squash",
			Usage:  "squash the layers at build time",
			EnvVar: "PLUGIN_SQUASH",
		},
		cli.BoolTFlag{
			Name:   "pull-image",
			Usage:  "force pull base image at build time",
			EnvVar: "PLUGIN_PULL_IMAGE",
		},
		cli.BoolFlag{
			Name:   "compress",
			Usage:  "compress the build context using gzip",
			EnvVar: "PLUGIN_COMPRESS",
		},
		cli.StringFlag{
			Name:   "repo",
			Usage:  "docker repository",
			EnvVar: "PLUGIN_REPO",
		},
		cli.StringSliceFlag{
			Name:   "custom-labels",
			Usage:  "additional k=v labels",
			EnvVar: "PLUGIN_CUSTOM_LABELS",
		},
		cli.StringSliceFlag{
			Name:   "label-schema",
			Usage:  "label-schema labels",
			EnvVar: "PLUGIN_LABEL_SCHEMA",
		},
		cli.BoolTFlag{
			Name:   "auto-label",
			Usage:  "auto-label true|false",
			EnvVar: "PLUGIN_AUTO_LABEL",
		},
		cli.StringFlag{
			Name:   "link",
			Usage:  "link https://example.com/org/repo-name",
			EnvVar: "PLUGIN_REPO_LINK,DRONE_REPO_LINK",
		},
		cli.StringFlag{
			Name:   "docker.registry",
			Usage:  "docker registry",
			Value:  "https://index.docker.io/v1/",
			EnvVar: "PLUGIN_REGISTRY,DOCKER_REGISTRY",
		},
		cli.StringFlag{
			Name:   "docker.username",
			Usage:  "docker username",
			EnvVar: "PLUGIN_USERNAME,DOCKER_USERNAME",
		},
		cli.StringFlag{
			Name:   "docker.password",
			Usage:  "docker password",
			EnvVar: "PLUGIN_PASSWORD,DOCKER_PASSWORD",
		},
		cli.StringFlag{
			Name:   "docker.email",
			Usage:  "docker email",
			EnvVar: "PLUGIN_EMAIL,DOCKER_EMAIL",
		},
		cli.StringFlag{
			Name:   "docker.config",
			Usage:  "docker json dockerconfig content",
			EnvVar: "PLUGIN_CONFIG,DOCKER_PLUGIN_CONFIG",
		},
		cli.BoolTFlag{
			Name:   "docker.purge",
			Usage:  "docker should cleanup images",
			EnvVar: "PLUGIN_PURGE",
		},
		cli.StringFlag{
			Name:   "repo.branch",
			Usage:  "repository default branch",
			EnvVar: "DRONE_REPO_BRANCH",
		},
		cli.BoolFlag{
			Name:   "no-cache",
			Usage:  "do not use cached intermediate containers",
			EnvVar: "PLUGIN_NO_CACHE",
		},
		cli.StringSliceFlag{
			Name:   "add-host",
			Usage:  "additional host:IP mapping",
			EnvVar: "PLUGIN_ADD_HOST",
		},
		cli.StringFlag{
			Name:   "s3-local-cache-dir",
			Usage:  "local directory for S3 based cache",
			EnvVar: "PLUGIN_S3_LOCAL_CACHE_DIR",
		},
		cli.StringFlag{
			Name:   "s3-bucket",
			Usage:  "S3 bucket name",
			EnvVar: "PLUGIN_S3_BUCKET",
		},
		cli.StringFlag{
			Name:   "s3-endpoint",
			Usage:  "S3 endpoint address",
			EnvVar: "PLUGIN_S3_ENDPOINT",
		},
		cli.StringFlag{
			Name:   "s3-region",
			Usage:  "S3 region",
			EnvVar: "PLUGIN_S3_REGION",
		},
		cli.StringFlag{
			Name:   "s3-key",
			Usage:  "S3 access key",
			EnvVar: "PLUGIN_S3_ACCESS_KEY",
		},
		cli.StringFlag{
			Name:   "s3-secret",
			Usage:  "S3 access secret",
			EnvVar: "PLUGIN_S3_SECRET",
		},
		cli.BoolFlag{
			Name:   "s3-use-ssl",
			Usage:  "Enable SSL for S3 connections",
			EnvVar: "PLUGIN_S3_USE_SSL",
		},
		cli.BoolFlag{
			Name:   "layers",
			Usage:  "User Layers",
			EnvVar: "PLUGIN_LAYERS",
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) error {
	plugin := docker.Plugin{
		Dryrun:  c.Bool("dry-run"),
		Cleanup: c.BoolT("docker.purge"),
		Login: docker.Login{
			Registry: c.String("docker.registry"),
			Username: c.String("docker.username"),
			Password: c.String("docker.password"),
			Email:    c.String("docker.email"),
			Config:   c.String("docker.config"),
		},
		Build: docker.Build{
			Remote:      c.String("remote.url"),
			Name:        c.String("commit.sha"),
			Dockerfile:  c.String("dockerfile"),
			Context:     c.String("context"),
			Tags:        c.StringSlice("tags"),
			Args:        c.StringSlice("args"),
			ArgsEnv:     c.StringSlice("args-from-env"),
			Target:      c.String("target"),
			Squash:      c.Bool("squash"),
			Pull:        c.BoolT("pull-image"),
			CacheFrom:   c.StringSlice("cache-from"),
			Compress:    c.Bool("compress"),
			Repo:        c.String("repo"),
			Labels:      c.StringSlice("custom-labels"),
			LabelSchema: c.StringSlice("label-schema"),
			AutoLabel:   c.BoolT("auto-label"),
			Link:        c.String("link"),
			NoCache:     c.Bool("no-cache"),
			AddHost:     c.StringSlice("add-host"),
			Quiet:       c.Bool("quiet"),
			S3CacheDir:  c.String("s3-local-cache-dir"),
			S3Bucket:    c.String("s3-bucket"),
			S3Endpoint:  c.String("s3-endpoint"),
			S3Region:    c.String("s3-region"),
			S3Key:       c.String("s3-key"),
			S3Secret:    c.String("s3-secret"),
			S3UseSSL:    c.Bool("s3-use-ssl"),
			Layers:      c.Bool("layers"),
		},
	}

	if c.Bool("tags.auto") {
		if docker.UseDefaultTag( // return true if tag event or default branch
			c.String("commit.ref"),
			c.String("repo.branch"),
		) {
			tag, err := docker.DefaultTagSuffix(
				c.String("commit.ref"),
				c.String("tags.suffix"),
			)
			if err != nil {
				logrus.Printf("cannot build docker image for %s, invalid semantic version", c.String("commit.ref"))
				return err
			}
			plugin.Build.Tags = tag
		} else {
			logrus.Printf("skipping automated docker build for %s", c.String("commit.ref"))
			return nil
		}
	}

	return plugin.Exec()
}
