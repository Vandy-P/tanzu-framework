package main

import (
	 "fmt"
	 "github.com/cppforlife/go-cli-ui/ui"
	        "github.com/vmware-tanzu/carvel-imgpkg/pkg/imgpkg/bundle"
        "github.com/vmware-tanzu/carvel-imgpkg/pkg/imgpkg/cmd"
        "github.com/vmware-tanzu/carvel-imgpkg/pkg/imgpkg/registry"

)


func CopyImageToTar (sourceImageName, destImageRepo, customImageRepoCertificate string, insecureconnection bool) error {
        confUI := ui.NewConfUI(ui.NewNoopLogger()) 
        copyOptions := cmd.NewCopyOptions(confUI)
        copyOptions.TarFlags.Resume = true
        copyOptions.IncludeNonDistributable = true
        copyOptions.Concurrency = 1
        copyOptions.RegistryFlags.Insecure = insecureconnection
        reg, err := registry.NewSimpleRegistry(registry.Opts{})
        if err != nil {
                return err
        }
        newBundle := bundle.NewBundle(sourceImageName, reg)
        isBundle, _ := newBundle.IsBundle()
        if isBundle {
                copyOptions.BundleFlags = cmd.BundleFlags{Bundle: sourceImageName}
                fmt.Printf("Bundle ==> sourceImageName  ====> %s desttar ===> %s \n", sourceImageName, destImageRepo)
        } else {
                copyOptions.ImageFlags = cmd.ImageFlags{Image: sourceImageName}
                fmt.Printf("Image ==>  sourceImageName  ====> %s desttar ===> %s \n", sourceImageName, destImageRepo)
        }
        copyOptions.TarFlags.TarDst = destImageRepo
        if customImageRepoCertificate != "" {
                copyOptions.RegistryFlags.CACertPaths = []string{customImageRepoCertificate}
        }
        err = copyOptions.Run()
        if err != nil {
                return err
        }
        return nil
}

func main()  {
	CopyImageToTar("projects-stg.registry.vmware.com/tkg/packages/core/azuredisk-csi-driver:v1.27.0_vmware.1-tkg.1-20230321", "packages-core-azuredisk-csi-driver-v1.27.0_vmware.1-tkg.1-20230321.tar", "", true)
	CopyImageToTar("projects-stg.registry.vmware.com/tkg/packages/core/repo:v1.25.6_vmware.1-tkg.1-20230321", "temp.tar", "", true)
}
