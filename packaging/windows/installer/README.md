# Windows Installer

## Current Implementation (v0)

The v0 installer uses a simple batch script that:
1. Copies the binary to `%LOCALAPPDATA%\Heimdex`
2. Creates a startup shortcut
3. Launches the agent

## Future Implementation (v1+)

For production, consider:
- **MSI Installer**: Use WiX Toolset for proper Windows Installer package
- **Windows Service**: Register as a Windows service instead of startup app
- **Code Signing**: Sign the executable with an EV certificate

### WiX Toolset Setup

```xml
<!-- Example WiX configuration -->
<Product Id="*" Name="Heimdex Agent" Version="0.1.0" 
         Manufacturer="Heimdex Inc." UpgradeCode="...">
  <Package InstallerVersion="200" Compressed="yes"/>
  <Media Id="1" Cabinet="heimdex.cab" EmbedCab="yes"/>
  
  <Directory Id="TARGETDIR" Name="SourceDir">
    <Directory Id="LocalAppDataFolder">
      <Directory Id="INSTALLDIR" Name="Heimdex">
        <Component Id="MainExecutable">
          <File Id="AgentExe" Source="heimdex-agent.exe" KeyPath="yes"/>
        </Component>
      </Directory>
    </Directory>
  </Directory>
  
  <Feature Id="Complete" Level="1">
    <ComponentRef Id="MainExecutable"/>
  </Feature>
</Product>
```

### Service Registration

To run as a Windows service:

```go
// Use golang.org/x/sys/windows/svc or kardianos/service
import "golang.org/x/sys/windows/svc"

func main() {
    isService, err := svc.IsWindowsService()
    if err != nil {
        log.Fatal(err)
    }
    if isService {
        svc.Run("heimdex-agent", &agentService{})
    } else {
        // Interactive mode
        run()
    }
}
```
