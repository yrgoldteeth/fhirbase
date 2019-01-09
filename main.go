package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli"
)

// AvailableSchemas contains all know FHIR versions
var AvailableSchemas = []string{
	"1.0.2", "1.1.0", "1.4.0",
	"1.6.0", "1.8.0", "3.0.1",
	"3.2.0", "3.3.0", "dev",
}

var Version = "unknown"
var BuildDate = "unknown"

const logo = ` (        )  (    (                   (
 )\ )  ( /(  )\ ) )\ )   (     (      )\ )
(()/(  )\())(()/((()/( ( )\    )\    (()/( (
 /(_))((_)\  /(_))/(_)))((_)((((_)(   /(_)))\
(_))_| _((_)(_)) (_)) ((_)_  )\ _ )\ (_)) ((_)
| |_  | || ||_ _|| _ \ | _ ) (_)_\(_)/ __|| __|
| __| | __ | | | |   / | _ \  / _ \  \__ \| _|
|_|   |_||_||___||_|_\ |___/ /_/ \_\ |___/|___|`

func main() {
	cli.AppHelpTemplate = fmt.Sprintf("%s\n\n%s", logo, cli.AppHelpTemplate)

	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s built at %s\n", Version, BuildDate)
	}

	app := cli.NewApp()
	app.Name = "fhirbase"
	app.Version = Version
	app.Usage = "command-line utility to operate on FHIR data with PostgreSQL database."

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "nostats",
			Usage:       "Disable sending of usage statistics",
			Destination: &DisableStats,
		},
		cli.StringFlag{
			Name:        "host, n",
			Value:       "localhost",
			Usage:       "PostgreSQL host",
			EnvVar:      "PGHOST",
			Destination: &PgConfig.Host,
		},
		cli.UintFlag{
			Name:        "port, p",
			Value:       5432,
			Usage:       "PostgreSQL port",
			EnvVar:      "PGPORT",
			Destination: &PgConfig.Port,
		},
		cli.StringFlag{
			Name:        "username, U",
			Value:       "postgres",
			Usage:       "PostgreSQL username",
			EnvVar:      "PGUSER",
			Destination: &PgConfig.Username,
		},
		cli.StringFlag{
			Name:        "sslmode, s",
			Value:       "prefer",
			Usage:       "PostgreSQL sslmode setting (disable/allow/prefer/require/verify-ca/verify-full)",
			EnvVar:      "PGSSLMODE",
			Destination: &PgConfig.SSLMode,
		},
		cli.StringFlag{
			Name:  "fhir, f",
			Value: "3.3.0",
			Usage: "FHIR version to use. Know FHIR versions are: " + strings.Join(AvailableSchemas, ", "),
		},
		cli.StringFlag{
			Name:        "db, d",
			Value:       "",
			Usage:       "Database to connect to",
			EnvVar:      "PGDATABASE",
			Destination: &PgConfig.Database,
		},
		cli.StringFlag{
			Name:        "password, W",
			Usage:       "PostgreSQL password",
			EnvVar:      "PGPASSWORD",
			Destination: &PgConfig.Password,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:      "init",
			HelpName:  "init",
			Hidden:    false,
			Usage:     "Creates Fhirbase schema in specific database",
			UsageText: "fhirbase [--fhir=FHIR version] [postgres connection options] init",
			Description: `
Creates SQL schema (tables, types and stored procedures) to store
resources from FHIR version specified with "--fhir" flag. Database
where schema will be created is specified with "--db" flag. Specified
database should be empty, otherwise command may fail with an SQL
error.`,
			Action: InitCommand,
		},
		{
			Name:      "transform",
			HelpName:  "transform",
			Hidden:    false,
			Usage:     "Performs Fhirbase transformation on a single FHIR resource loaded from a JSON file",
			UsageText: "fhirbase [--fhir=FHIR version] transform path/to/fhir-resource.json",
			Description: `
Transform command applies Fhirbase transformation algorithm to a
single FHIR resource loaded from provided JSON file and outputs result
to the STDOUT. This command exists mostly for demonstration and
debugging of Fhirbase transformation logic.

For detailed explanation of Fhirbase transformation algorithm please
proceed to the Fhirbase documentation. TODO: direct documentation
link.`,
			Action: TransformCommand,
		},
		{
			Name:      "bulkget",
			HelpName:  "bulkget",
			Hidden:    false,
			ArgsUsage: "[BULK DATA ENDPOINT] [TARGET DIR]",
			Usage:     "Downloads FHIR data from Bulk Data API endpoint and saves NDJSON files on local filesystem into specified directory",
			UsageText: "fhirbase bulkget [--numdl=10] http://some-fhir-server.com/fhir/Patient/$everything /output/dir/",
			Description: `
Downloads FHIR data from Bulk Data API endpoint and saves results into
specific directory on a local filesystem.

NDJSON files generated by remote server will be downloaded in
parallel and you can specify number of threads with "--numdl" flag.

To mitigate differences between Bulk Data API implementations, there
is an "--accept-header" option which sets the value for "Accept"
header. Most likely you won't need to set it, but if Bulk Data server
rejects queries because of "Accept" header value, consider explicitly
set it to something it expects.
`,
			Action: BulkGetCommand,
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:  "numdl",
					Value: 5,
					Usage: "Number of parallel downloads for Bulk Data API client",
				},
				cli.StringFlag{
					Name:  "accept-header",
					Value: "application/fhir+json",
					Usage: "Value for Accept HTTP header (i.e. 'application/ndjson' for Cerner implementation)",
				},
			},
		},
		{
			Name:      "load",
			HelpName:  "load",
			Hidden:    false,
			Usage:     "Loads FHIR resources into database",
			ArgsUsage: "[BULK DATA URL or FILE PATH(s)]",
			Description: `
Load command loads FHIR resources from named source(s) into the
Fhirbase database.

You can provide either single Bulk Data URL or several file paths as
an input.

Fhirbase can read from following file types:

  * NDJSON files
  * transaction or collection FHIR Bundles
  * regular JSON files containing single FHIR resource

Also Fhirbase can read gziped files, so all of the supported file
formats can be additionally gziped.

You are allowed to mix different file formats and gziped/non-gziped
files in a single command input, i.e.:

  fhirbase load *.ndjson.gzip patient-john-doe.json my-tx-bundle.json

Fhirbase automatically detects gzip compression and format of the
input file, so you don't have to provide any additional hints. Even
file name extensions can be ommited, because Fhirbase analyzes file
content, not the file name.

If Bulk Data URL was provided, Fhirbase will download NDJSON files
first (see the help for "bulkget" command) and then load them as a
regular local files. Load command accepts all the command-line flags
accepted by bulkget command.

Fhirbase reads input files sequentially, reading single resource at a
time. And because of PostgreSQL traits it's important if Fhirbase gets
a long enough series of resources of the same type from the provided
input, or it gets resource of a different type on every next read. We
will call those two types of inputs "grouped" and "non-grouped",
respectively. Good example of grouped input is NDJSON files produced
by Bulk Data API server. A set of JSON files from FHIR distribution's
"examples" directory is an example of non-grouped input. Because
Fhirbase reads resources one by one and do not load the whole file, it
cannot know if you provided grouped or non-grouped input.

Fhirbase supports two modes (or methods) to put resources into the
database: "insert" and "copy". Insert mode uses INSERT statements and
copy mode uses COPY FROM STDIN. By default, Fhirbase uses insert mode
for local files and copy mode for Bulk Data API loads.

It does not matter for insert mode if your input is grouped or not. It
will perform with same speed on both. Use it when you're not sure what
type of input you have. Also insert mode is useful when you have
duplicate IDs in your source files (rare case but happened couple of
times). Insert mode will ignore duplicates and will persist only the
first occurrence of a specific resource instance, ignoring other
occurrences.

Copy mode is intended to be used only with grouped inputs. When
applied to grouped inputs, it's almost 3 times faster than insert
mode. But it's same slower if it's being applied to non-grouped
input.`,
			Action: LoadCommand,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "mode, m",
					Value: "insert",
					Usage: "Load mode to use, possible values are 'copy' and 'insert'",
				},
				cli.UintFlag{
					Name:  "numdl",
					Value: 5,
					Usage: "Number of parallel downloads for Bulk Data API client",
				},
				cli.BoolFlag{
					Name:  "memusage",
					Usage: "Outputs memory usage during resources loading (for debug purposes)",
				},
				cli.StringFlag{
					Name:  "accept-header",
					Value: "application/fhir+json",
					Usage: "Value for Accept HTTP header (should be application/ndjson for Cerner, application/fhir+json for Smart)",
				},
			},
		},
		{
			Name:      "web",
			HelpName:  "web",
			Hidden:    false,
			Usage:     "Starts web server with primitive UI to perform SQL queries from the browser",
			ArgsUsage: "",
			Description: `
Starts a simple web server to invoke SQL queries from the browser UI.

You can specify web server's host and port with "--webhost" and
"--webport" flags. If "--webhost" flag is empty (set to blank string)
then web server will listen on all available network interfaces.`,
			Action: WebCommand,
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:  "webport",
					Value: 3000,
					Usage: "Port to start webserver on",
				},
				cli.StringFlag{
					Name:  "webhost",
					Value: "",
					Usage: "Host to start webserver on",
				},
			},
		},
		{
			Name:      "update",
			HelpName:  "update",
			Hidden:    false,
			Usage:     "Updates Fhirbase to most recent version",
			ArgsUsage: "",
			Description: `
Updates Fhirbase to most recent version.

If you currently use nightly build (Fhirbase version starts with
'nightly-' and commit hash), it will update Fhirbase to most recent
nightly build. Otherwise it will update to most recent stable
version.`,
			Action: updateCommand}}

	app.Action = func(c *cli.Context) error {
		cli.HelpPrinter(os.Stdout, cli.AppHelpTemplate, app)
		return nil
	}

	err := app.Run(os.Args)

	if err != nil {
		submitErrorEvent(err)
		fmt.Printf("%+v\n", err)

		waitForAllEventsSubmitted()
		os.Exit(1)
	}

	waitForAllEventsSubmitted()
	os.Exit(0)
}
