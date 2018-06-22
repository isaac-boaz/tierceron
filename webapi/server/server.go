package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
)

//Server implements the twirp api server endpoints
type Server struct {
	VaultToken string
	VaultAddr  string
	CertPath   string
}

//ListServiceTemplates lists the templates under the requested service
func (s *Server) ListServiceTemplates(ctx context.Context, req *pb.ListReq) (*pb.ListResp, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		return nil, err
	}

	listPath := "templates/" + req.Service
	secret, err := mod.List(listPath)
	if err != nil {
		return nil, err
	}

	if len(secret.Warnings) > 0 {
		for _, warning := range secret.Warnings {
			fmt.Printf("Warning: %s\n", warning)
		}
		return nil, errors.New("Warnings generated from vault " + req.Service)
	}

	fileNames := []string{}
	for _, fileName := range secret.Data["keys"].([]interface{}) {
		if strFile, ok := fileName.(string); ok {
			if strFile[len(strFile)-1] != '/' { // Skip subdirectories where template files are stored
				fileNames = append(fileNames, strFile)
			}
		}
	}

	return &pb.ListResp{
		Templates: fileNames,
	}, nil
}

// GetTemplate makes a request to the vault for the template found in <service>/<file>/template-file
// Returns the template data in base64 and the template's extension. Returns any errors generated by vault
func (s *Server) GetTemplate(ctx context.Context, req *pb.TemplateReq) (*pb.TemplateResp, error) {
	// Connect to the vault
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		return nil, err
	}

	// Get template data from information in request.
	path := "templates/" + req.Service + "/" + req.File + "/template-file"
	data, err := mod.ReadData(path)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, errors.New("No file " + req.File + " under " + req.Service)
	}

	// Return retrieved data in response
	return &pb.TemplateResp{
		Data: data["data"].(string),
		Ext:  data["ext"].(string)}, nil
}

// Validate checks the vault to see if the requested credentials are validated
func (s *Server) Validate(ctx context.Context, req *pb.ValidationReq) (*pb.ValidationResp, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		return nil, err
	}
	mod.Env = req.Env

	servicePath := "verification/" + req.Service
	data, err := mod.ReadData(servicePath)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, errors.New("No verification for " + req.Service + " found under " + req.Env + " environment")
	}

	return &pb.ValidationResp{IsValid: data["verified"].(bool)}, nil
}

func (s *Server) MakeVault(ctx context.Context, req *pb.MakeVaultReq) (*pb.Vault, error) {
	// Connect to the vault
	//mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	//mod.Env = req.Env
	//fmt.Println("mod: " + mod.Env)
	// if err != nil {
	// 	return nil, err
	// }
	environments := []*apinator.Vault_Env{}
	envStrings := []string{"dev", "QA", "local", "secrets"}
	for _, environment := range envStrings {
		mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
		mod.Env = environment
		if err != nil {
			return nil, err
		}
		services := []*apinator.Vault_Env_Service{}
		//get a list of services under values
		servicePaths := getPaths(mod, "values/")
		for _, servicePath := range servicePaths {
			files := []*apinator.Vault_Env_Service_File{}
			//get a list of files under service
			filePaths := getPaths(mod, servicePath)
			for _, filePath := range filePaths {
				vals := []*apinator.Vault_Env_Service_File_Value{}
				//get a list of values
				valueMap, err := mod.ReadData(filePath)
				if err != nil {
					panic(err)
				}
				if valueMap != nil {
					//fmt.Println("data at path " + path)
					for key, value := range valueMap {
						kv := &apinator.Vault_Env_Service_File_Value{Key: key, Value: value.(string)}
						vals = append(vals, kv)
						//data = append(data, value.(string))
					}

				}
				file := &apinator.Vault_Env_Service_File{Name: getPathEnd(filePath), Values: vals}
				files = append(files, file)
			}
			service := &apinator.Vault_Env_Service{Name: getPathEnd(servicePath), Files: files}
			services = append(services, service)
		}
		env := &apinator.Vault_Env{Name: environment, Services: services}
		environments = append(environments, env)
	}
	//vault := Vault{Envs: environments}
	//from each environment(dev, QA, secrets, local)
	//get the services
	//get the values
	return &pb.Vault{
		Envs: environments,
	}, nil
}
func getPaths(mod *kv.Modifier, pathName string) []string {
	secrets, err := mod.List(pathName)
	pathList := []string{}
	if err != nil {
		panic(err)
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})
		//fmt.Println("secrets are")
		//fmt.Println(slicey)
		for _, pathEnd := range slicey {
			//List is returning both pathEnd and pathEnd/
			path := pathName + pathEnd.(string)
			pathList = append(pathList, path)
		}
		return pathList
	}
	return pathList
}
func getPathEnd(path string) string {
	strs := strings.Split(path, "/")
	for strs[len(strs)-1] == "" {
		strs = strs[:len(strs)-1]
	}
	return strs[len(strs)-1]
}
