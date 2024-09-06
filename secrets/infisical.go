package secrets

import (
	infisical "github.com/infisical/go-sdk"
	"github.com/infisical/go-sdk/packages/models"
	"log"
	"log/slog"
	"sync"
)

//
//apiKeySecret, err := client.Secrets().Retrieve(infisical.RetrieveSecretOptions{
//SecretKey:   "CF_ZONE_ID",
//Environment: "dev",
//ProjectID:   "596b4dd8-18ef-4b94-ada8-a26d00734dbc",
//SecretPath:  "/Cloudflare/",
//})
//
//if err != nil {
//fmt.Printf("Error: %v", err)
//os.Exit(1)
//}
//
//fmt.Println("Secret:", apiKeySecret.SecretValue)

type SecretManager interface {
	Get(secretPath string, secretKey string) (models.Secret, error)
	//List(secretPath string, environment string) ([]*models.Secret, error)
	ListFolders(secretPath string) ([]models.Folder, error)
	ListSecrets(secretPath string) ([]models.Secret, error)
	LoadSecrets() error
}

// Configs are used to create the deployment client
type InfisicalConfig struct {
	ClientId     string
	ClientSecret string
	ProjectId    string
	Environment  string
}

type infisicalCmd struct {
	client             infisical.InfisicalClientInterface
	attachToProcessEnv bool
	projectId          string
	Environment        string
	EncryptionKey      string
}

// NewClient will return a deployment image builder client
func NewClientSecret(infisicalConfig InfisicalConfig) (SecretManager, error) {

	client := infisical.NewInfisicalClient(infisical.Config{})

	// Authenticate with Infisical
	_, err := client.Auth().UniversalAuthLogin(infisicalConfig.ClientId, infisicalConfig.ClientSecret)

	if err != nil {
		log.Panicln("Authentication to Infisical failed: %v", err)
	}

	encryptionKey, _ := GenerateKey()

	secretManager := &infisicalCmd{
		client:             client,
		attachToProcessEnv: true,
		projectId:          infisicalConfig.ProjectId,
		Environment:        infisicalConfig.Environment,
		EncryptionKey:      encryptionKey,
	}

	return secretManager, nil
}

func (secretManager *infisicalCmd) Get(secretPath string, secretKey string) (models.Secret, error) {

	secret, err := secretManager.client.Secrets().Retrieve(infisical.RetrieveSecretOptions{
		SecretKey:   secretKey,
		Environment: secretManager.Environment,
		ProjectID:   secretManager.projectId,
		SecretPath:  secretPath,
	})

	return secret, err
}

func (secretManager *infisicalCmd) LoadSecrets() error {

	folders, err := secretManager.ListFolders("/")

	listFoldersRecursive := []string{"/"}

	for _, folder := range folders {
		listFoldersRecursive = append(listFoldersRecursive, "/"+folder.Name)

		// We can have 2 levels max
		subfolders, _ := secretManager.ListFolders("/" + folder.Name)
		if len(subfolders) > 0 {
			for _, subfolder := range subfolders {
				listFoldersRecursive = append(listFoldersRecursive, "/"+folder.Name+"/"+subfolder.Name)
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(listFoldersRecursive))

	for _, folder := range listFoldersRecursive {
		//log.Println("Loading secrets from folder: ", folder)
		go loadSecretInEnv(secretManager, folder, &wg)
	}
	wg.Wait()

	if err != nil {
		log.Panicf("Error: %v", err)
	}

	return nil
}

func (secretManager *infisicalCmd) ListFolders(secretPath string) ([]models.Folder, error) {

	folders, err := secretManager.client.Folders().List(infisical.ListFoldersOptions{
		ProjectID:   secretManager.projectId,
		Environment: secretManager.Environment,
		Path:        secretPath,
	})

	return folders, err
}

func (secretManager *infisicalCmd) ListSecrets(secretPath string) ([]models.Secret, error) {

	secrets, err := secretManager.client.Secrets().List(infisical.ListSecretsOptions{
		ProjectID:          secretManager.projectId,
		Environment:        secretManager.Environment,
		SecretPath:         secretPath,
		AttachToProcessEnv: true,
	})

	return secrets, err
}

func loadSecretInEnv(secretManager *infisicalCmd, secretPath string, wg *sync.WaitGroup) {
	defer wg.Done()
	secrets, _ := secretManager.ListSecrets(secretPath)

	// Create an array of all the secret names
	var secretNames []string
	for _, secret := range secrets {
		secretNames = append(secretNames, secret.SecretKey)
	}

	//log.Printf("--> Secrets loaded from %v: %v secrets: %v", secretPath, len(secrets), secretNames)
	//log.Printf("--> Secrets loaded from %v: %v", secretPath, len(secrets))
	slog.Debug("Secrets loaded from %v: %v secrets: %v", secretPath, len(secrets), secretNames)
}
