package run

import (
	"bgos/internal/boundary"
	"bgos/internal/globals"
	"bgos/internal/gsuite"
	"context"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strings"
	"time"
)

const (
	descriptionShort = `Execute synchronization process`

	descriptionLong = `
	Run execute synchronization process`

	//

	//
	LogLevelFlagErrorMessage                   = "impossible to get flag --log-level: %s"
	DisableTraceFlagErrorMessage               = "impossible to get flag --disable-trace: %s"
	GroupFlagErrorMessage                      = "impossible to get flag --group: %s"
	SyncTimeFlagErrorMessage                   = "impossible to get flag --sync-time: %s"
	UnableParseDurationErrorMessage            = "unable to parse duration: %s"
	UnableCreateGroupErrorMessage              = "unable to create group in Boundary: %s"
	UnableSetGroupMembersErrorMessage          = "unable to set group members: %s"
	EnvironmentVariableErrorMessage            = "environment variable not found"
	GsuiteCreateAdminErrorMessage              = "Unable to create new admin: %s"
	BoundaryAuthMethodNotSupportedErrorMessage = "boundary auth method not supported"
	BoundaryOidcIdFlagErrorMessage             = "impossible to get flag --boundary-oidc-id: %s"
	BoundaryScopeIdFlagErrorMessage            = "impossible to get flag --boundary-scope-id: %s"
	GetBoundaryPersonMapErrorMessage           = "Unable to get boundary persons: %s"
	SetupBoundaryObjErrorMessage               = "Fail to setup boundary object: %s"
	BoundaryOidcIdRequiredFlagErrorMessage     = "Mark boundary-oidc-id flag as required fail: %s"
	UnableBoundaryGetGroupsErrorMessage        = "Unable to get boundary groups: %s"
)

var (
	bAddressEnv            = os.ExpandEnv(os.Getenv("BOUNDARY_ADDR"))
	bAuthMethodPassIdEnv   = os.ExpandEnv(os.Getenv("BOUNDARY_AUTHMETHODPASS_ID"))
	bAuthMethodPassUserEnv = os.ExpandEnv(os.Getenv("BOUNDARY_AUTHMETHODPASS_USER"))
	bAuthMethodPassPassEnv = os.ExpandEnv(os.Getenv("BOUNDARY_AUTHMETHODPASS_PASS"))
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "run",
		DisableFlagsInUseLine: true,
		Short:                 descriptionShort,
		Long:                  descriptionLong,

		Run: RunCommand,
	}

	//
	cmd.Flags().String("log-level", "info", "Verbosity level for logs")
	cmd.Flags().Bool("disable-trace", true, "Disable showing traces in logs")
	cmd.Flags().String("sync-time", "10m", "Waiting time between group synchronizations (in duration type)")
	cmd.Flags().String("google-sa-credentials-path", "google.json", "Google ServiceAccount credentials JSON file path")
	cmd.Flags().StringSlice("google-group", []string{}, "(Repeatable or comma-separated list) G.Workspace groups")
	cmd.Flags().String("boundary-oidc-id", "amoidc_changeme", "Boundary oidc auth method ID to compare its users against G.Workspace")
	cmd.Flags().String("boundary-scope-id", "global", "Boundary scope ID where the users and groups are synchronized")

	return cmd
}

// RunCommand TODO
// Ref: https://pkg.go.dev/github.com/spf13/pflag#StringSlice
func RunCommand(cmd *cobra.Command, args []string) {

	// Init the logger
	logLevelFlag, err := cmd.Flags().GetString("log-level")
	if err != nil {
		log.Fatalf(LogLevelFlagErrorMessage, err)
	}

	disableTraceFlag, err := cmd.Flags().GetBool("disable-trace")
	if err != nil {
		log.Fatalf(DisableTraceFlagErrorMessage, err)
	}

	err = globals.SetLogger(logLevelFlag, disableTraceFlag)
	if err != nil {
		log.Fatal(err)
	}

	// Conditions for the flags
	err = cmd.MarkFlagRequired("boundary-oidc-id")
	if err != nil {
		globals.ExecContext.Logger.Fatalf(BoundaryOidcIdRequiredFlagErrorMessage, err)
	}

	syncTime, err := cmd.Flags().GetString("sync-time")
	if err != nil {
		globals.ExecContext.Logger.Fatalf(SyncTimeFlagErrorMessage, err)
	}

	// Flags retrieval
	jsonFilepath, err := cmd.Flags().GetString("google-sa-credentials-path")
	if err != nil {
		globals.ExecContext.Logger.Fatalf(LogLevelFlagErrorMessage, err)
	}

	groupList, err := cmd.Flags().GetStringSlice("google-group")
	if err != nil {
		log.Fatalf(GroupFlagErrorMessage, err)
	}

	bOidcId, err := cmd.Flags().GetString("boundary-oidc-id")
	if err != nil {
		globals.ExecContext.Logger.Fatalf(BoundaryOidcIdFlagErrorMessage, err)
	}

	bScopeId, err := cmd.Flags().GetString("boundary-scope-id")
	if err != nil {
		globals.ExecContext.Logger.Fatalf(BoundaryScopeIdFlagErrorMessage, err)
	}

	if bAddressEnv == "" || bAuthMethodPassIdEnv == "" || bAuthMethodPassUserEnv == "" || bAuthMethodPassPassEnv == "" {
		globals.ExecContext.Logger.Fatalf(EnvironmentVariableErrorMessage)
	}

	if !strings.HasPrefix(bAuthMethodPassIdEnv, "ampw_") {
		globals.ExecContext.Logger.Fatalf(BoundaryAuthMethodNotSupportedErrorMessage)
	}

	/////////////////////////////
	// EXECUTION FLOW RELATED
	/////////////////////////////

	// Ref: https://github.com/hashicorp/boundary/blob/main/api/
	boundaryObj := boundary.Boundary{
		Ctx: globals.ExecContext.Context,

		Address:          bAddressEnv,
		ScopeId:          bScopeId,
		AuthMethodOidcId: bOidcId,

		AuthMethodPasswordId: bAuthMethodPassIdEnv,
		Username:             bAuthMethodPassUserEnv,
		Password:             bAuthMethodPassPassEnv,
	}

	globals.ExecContext.Logger.Infof("Init Boundary and Gsuite clients")
	err = boundaryObj.InitBoundary()
	if err != nil {
		globals.ExecContext.Logger.Fatalf(SetupBoundaryObjErrorMessage, err)
	}

	gsuiteCli, err := gsuite.NewAdmin(context.Background(), jsonFilepath)
	if err != nil {
		globals.ExecContext.Logger.Fatalf(GsuiteCreateAdminErrorMessage, err)
	}

	for {
		globals.ExecContext.Logger.Infof("Getting members from G.Workspace user-defined groups")
		gsuiteGroupList, err := gsuiteCli.GetGroupsMembers(groupList)

		globals.ExecContext.Logger.Infof("Getting user accounts from Boundary OIDC auth method")
		boundaryPersonMap, err := boundaryObj.GetPersonMap()
		if err != nil {
			globals.ExecContext.Logger.Fatalf(GetBoundaryPersonMapErrorMessage, err.Error())
		}

		globals.ExecContext.Logger.Infof("Getting groups from Boundary instance")
		bGroupMap, err := boundaryObj.GetGroups()
		if err != nil {
			globals.ExecContext.Logger.Fatalf(UnableBoundaryGetGroupsErrorMessage, err.Error())
		}

		for _, gGroup := range gsuiteGroupList {

			globals.ExecContext.Logger.Infof("Syncing group '%s' in Boundary", gGroup.Group)

			// Create groups in Boundary when they are missing
			if _, groupCreated := bGroupMap[gGroup.Group]; !groupCreated {
				globals.ExecContext.Logger.Infof("Creating missing group '%s' in Boundary", gGroup.Group)
				bGroupCreateResult, err := boundaryObj.CreateGroup(gGroup.Group)
				if err != nil {
					globals.ExecContext.Logger.Warnf(UnableCreateGroupErrorMessage, err.Error())
					continue
				}
				bGroupMap[gGroup.Group] = &boundary.Group{
					Id:      bGroupCreateResult.Item.Id,
					Name:    bGroupCreateResult.Item.Name, // TODO: Evaluate if needed
					Version: bGroupCreateResult.Item.Version,
					Members: []string{},
				}
			}

			// Check users between G.Workspace and H.Boundary and sync them (authoritative approach)
			globals.ExecContext.Logger.Infof("Checking users between Boundary and %s", gGroup.Group)
			var bGroupSurvivingMembers []string
			for _, gUser := range gGroup.Users {
				if _, bPersonFound := boundaryPersonMap[gUser]; bPersonFound {
					bGroupSurvivingMembers = append(bGroupSurvivingMembers, boundaryPersonMap[gUser].UserId)
				}
			}

			// Set users in the boundary group removing the members that are not part of the original group in gsuite
			globals.ExecContext.Logger.Infof("Setting group memberships for group '%s' in Boundary", gGroup.Group)
			err = boundaryObj.SetGroupMembers(bGroupMap[gGroup.Group].Id, bGroupMap[gGroup.Group].Version, bGroupSurvivingMembers)
			if err != nil {
				globals.ExecContext.Logger.Warnf(UnableSetGroupMembersErrorMessage, err.Error())
			}
		}

		//
		duration, err := time.ParseDuration(syncTime)
		if err != nil {
			globals.ExecContext.Logger.Fatalf(UnableParseDurationErrorMessage, err)
		}
		globals.ExecContext.Logger.Infof("Syncing again in %s", duration.String())
		time.Sleep(duration)
	}
}
