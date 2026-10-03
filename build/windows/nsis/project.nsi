Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows you to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
## 
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the wails_tools.nsh file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "my-project" # Default "client"
## !define INFO_COMPANYNAME    "My Company" # Default "My Company"
## !define INFO_PRODUCTNAME    "My Product Name" # Default "My Product"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "0.1.0"
## !define INFO_COPYRIGHT      "(c) Now, My Company" # Default "© 2026, My Company"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
## !define WAILS_INSTALL_SCOPE     "user"             # Default "machine" - set to "user" for per-user install ($LOCALAPPDATA) without UAC prompt
####
## Include the wails tools
####
# Per user, so in-app updates install without UAC.
!ifndef WAILS_INSTALL_SCOPE
    !define WAILS_INSTALL_SCOPE "user"
!endif
!include "wails_tools.nsh"
!include "StrFunc.nsh"
${StrStr}

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

Var Updating
Var FinishText

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

# SignPath requires the update check to be disclosed during installation.
# MUI 1 writes page texts into an InstallOptions INI, so newlines are \r\n, not $\r$\n.
!define MUI_WELCOMEPAGE_TEXT "Setup will install Tetiva for your Windows user.\r\n\r\nTetiva checks for a new version at most once a day by requesting a small file from api.tetiva.app. The request carries only the app version and operating system, nothing about you. New versions download in the background and install when you click Restart to update. Both can be turned off in Settings, under Updates.\r\n\r\nClick Next to continue."
!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!define MUI_FINISHPAGE_TEXT "$FinishText"
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
!else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
   StrCpy $FinishText "Tetiva has been installed on your computer.\r\n\r\nClick Finish to close Setup."
   ${GetParameters} $R0
   ClearErrors
   ${GetOptions} $R0 "/UPDATE" $R1
   ${IfNot} ${Errors}
       StrCpy $Updating 1
   ${EndIf}
FunctionEnd

Function WaitForApp
    ${IfNot} ${FileExists} "$INSTDIR\${PRODUCT_EXECUTABLE}"
        Return
    ${EndIf}
    ${For} $1 1 120
        ClearErrors
        FileOpen $0 "$INSTDIR\${PRODUCT_EXECUTABLE}" a
        ${IfNot} ${Errors}
            FileClose $0
            Return
        ${EndIf}
        Sleep 500
    ${Next}
    SetErrorLevel 2
    Quit
FunctionEnd

Function CloseRunningApp
    ${If} ${Silent}
        Return
    ${EndIf}
    retry:
    nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq ${PRODUCT_EXECUTABLE}" /NH'
    Pop $0
    Pop $1
    ${StrStr} $2 $1 "${PRODUCT_EXECUTABLE}"
    ${If} $2 != ""
        # tasklist also matches another user's Tetiva or another program's client.exe, hence Ignore.
        MessageBox MB_ABORTRETRYIGNORE "Close Tetiva to continue." IDRETRY retry IDIGNORE done
        Abort
    ${EndIf}
    done:
FunctionEnd

# The uninstaller relaunches itself from %TEMP%, so ExecShellWait returns before it is done.
!macro RemoveMachineCopy KEY
    ReadRegStr $0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${KEY}" "UninstallString"
    ReadRegStr $2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${KEY}" "DisplayIcon"
    ${If} $0 != ""
        StrCpy $1 $0 1
        ${If} $1 == '"'
            StrCpy $0 $0 -1 1
        ${EndIf}
        ClearErrors
        ExecShellWait "open" "$0" "/S"
        ${IfNot} ${Errors}
            ${For} $1 1 120
                ReadRegStr $0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${KEY}" "UninstallString"
                ${If} $0 == ""
                ${AndIfNot} ${FileExists} $2
                    ${ExitFor}
                ${EndIf}
                Sleep 500
            ${Next}
        ${EndIf}
        ${If} $0 != ""
        ${OrIf} ${FileExists} $2
            StrCpy $FinishText "Tetiva has been installed. An older copy is still in Program Files; you can remove it in Settings > Apps.\r\n\r\nClick Finish to close Setup."
        ${EndIf}
    ${EndIf}
!macroend

Function MigrateMachineInstall
    SetRegView 64
    !insertmacro RemoveMachineCopy "Saveliy YudinTetiva"
    !insertmacro RemoveMachineCopy "Saveliy LudinTetiva"
FunctionEnd

Function .onInstSuccess
    ${If} $Updating == 1
        Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
    ${EndIf}
FunctionEnd

Section
    !insertmacro wails.setShellContext

    ${If} $Updating == 1
        Call WaitForApp
    ${Else}
        Call CloseRunningApp
        Call MigrateMachineInstall
    ${EndIf}

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    
    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    # An update must not bring back a desktop icon the user deleted.
    ${If} $Updating != 1
        CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${EndIf}

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    
    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall" 
    !insertmacro wails.setShellContext

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
