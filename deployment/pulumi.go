package deployment

//
//import (
//	"context"
//	"fmt"
//	"github.com/pulumi/pulumi/sdk/go/pulumi"
//	"github.com/pulumi/pulumi/sdk/v3/go/auto"
//	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
//)
//
//func deployWithPulumi(config DeploymentRequest) chan error {
//	resultChan := make(chan error)
//
//	go func() {
//		ctx := context.Background()
//		projectName := "deployment-project"
//		stackName := "dev"
//
//		// Create or select a stack
//		s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, func(ctx *pulumi.Context) error {
//			// Create your Pulumi program here using the provided config
//			// For example, to create a Kubernetes Deployment, use the config values
//			// Example is not fully implemented, you need to fill the resource creation part
//			return nil
//		})
//		if err != nil {
//			resultChan <- fmt.Errorf("failed to create or select stack: %v", err)
//			return
//		}
//
//		// Set up Pulumi stack configuration
//		// Add any necessary configuration settings here
//
//		// Run Pulumi update
//		_, err = s.Up(ctx, optup.Message("Updating stack"))
//		if err != nil {
//			resultChan <- fmt.Errorf("failed to update stack: %v", err)
//			return
//		}
//
//		resultChan <- nil
//	}()
//
//	return resultChan
//}
