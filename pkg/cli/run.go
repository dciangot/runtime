package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/goombaio/namegenerator"
	"github.com/ibuildthecloud/baaah/pkg/typed"
	v1 "github.com/ibuildthecloud/herd/pkg/apis/herd-project.io/v1"
	"github.com/ibuildthecloud/herd/pkg/appdefinition"
	"github.com/ibuildthecloud/herd/pkg/client"
	"github.com/ibuildthecloud/herd/pkg/flagparams"
	"github.com/ibuildthecloud/herd/pkg/run"
	"github.com/rancher/wrangler-cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	nameGenerator = namegenerator.NewNameGenerator(time.Now().UnixNano())
)

func NewRun() *cobra.Command {
	cmd := cli.Command(&Run{}, cobra.Command{
		Use:          "run [flags] IMAGE",
		SilenceUsage: true,
		Short:        "Run an app from an app image",
		Args:         cobra.MinimumNArgs(1),
	})
	cmd.Flags().SetInterspersed(false)
	return cmd
}

type Run struct {
	Name        string   `usage:"Name of app to create" short:"n"`
	Publish     []string `usage:"Published a container to a friendly domain (format public:private) (ex: example.com:web)" short:"p"`
	PullSecrets []string `usage:"Secret names to authenticate pull images in cluster" short:"l"`
	Volumes     []string `usage:"Bind an existing volume (format existing:vol-name) (ex: pvc-name:app-data)" short:"v"`
	Secrets     []string `usage:"Bind an existing secret (format existing:sec-name) (ex: sec-name:app-secret)" short:"s"`
}

func (s *Run) getName() (string, bool) {
	if s.Name != "" {
		return "", false
	}
	return nameGenerator.Generate(), true
}

func usage(app *v1.AppSpec) func() {
	return func() {
		fmt.Println()
		if len(app.Volumes) == 0 {
			fmt.Println("Volumes: <none>")
		} else {
			fmt.Print("Volumes: ")
			fmt.Println(strings.Join(typed.Keys(app.Volumes), ", "))
		}

		if len(app.Secrets) == 0 {
			fmt.Println("Secrets: <none>")
		} else {
			fmt.Print("Secrets: ")
			fmt.Println(strings.Join(typed.Keys(app.Secrets), ", "))
		}

		if len(app.Secrets) == 0 {
			fmt.Println("Container: <none>")
		} else {
			fmt.Print("Container: ")
			fmt.Println(strings.Join(typed.Keys(app.Containers), ", "))
		}
		fmt.Println()
	}
}

func (s *Run) Run(cmd *cobra.Command, args []string) error {
	c, err := client.Default()
	if err != nil {
		return err
	}

	image := args[0]

	appImage, err := c.GetAppImage(cmd.Context(), image, s.PullSecrets)
	if err != nil {
		return err
	}

	appDef, err := appdefinition.FromAppImage(appImage)
	if err != nil {
		return err
	}

	appSpec, err := appDef.AppSpec()
	if err != nil {
		return err
	}

	params, err := appDef.DeployParams()
	if err != nil {
		return err
	}

	flags := flagparams.New(image, params)
	flags.Usage = usage(appSpec)

	deployParams, err := flags.Parse(args)
	if pflag.ErrHelp == err {
		return nil
	} else if err != nil {
		return err
	}

	opts := client.AppRunOptions{
		Name:             s.Name,
		ImagePullSecrets: s.PullSecrets,
		DeployParams:     deployParams,
	}

	opts.Endpoints, err = run.ParseEndpoints(s.Publish)
	if err != nil {
		return err
	}

	opts.Volumes, err = run.ParseVolumes(s.Volumes)
	if err != nil {
		return err
	}

	opts.Secrets, err = run.ParseSecrets(s.Secrets)
	if err != nil {
		return err
	}

	app, err := c.AppRun(cmd.Context(), image, &opts)
	if err != nil {
		return err
	}

	fmt.Println(app.Name)
	return nil
}
