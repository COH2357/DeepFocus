; DeepFocus installer (NSIS 3.x, per-user, no elevation).
;
; DeepFocus is a fork of Focusd. The install/data directory deliberately stays
; at %APPDATA%\focusd so existing users keep their database, config and history.

!ifndef VERSION
  !define VERSION "0.0.0"
!endif

!include "MUI2.nsh"
!include "WinMessages.nsh"

RequestExecutionLevel user

Name "DeepFocus"
OutFile "deepfocus_setup.exe"
InstallDir "$APPDATA\focusd"

Var PsExe

!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\DeepFocus"
!define LEGACY_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Focusd"

!define MUI_ABORTWARNING
!define MUI_ICON "assets\focusd.ico"
!define MUI_UNICON "assets\focusd.ico"
!define MUI_WELCOMEPAGE_TITLE "Install or upgrade DeepFocus"
!define MUI_WELCOMEPAGE_TEXT "Setup installs DeepFocus for the current user.$\r$\n$\r$\nIf an older DeepFocus or Focusd is already installed, it is upgraded in place. Your database, settings and history are kept; only the program files are replaced.$\r$\n$\r$\nThe installer stops the running tracker and desktop app before copying files."

!insertmacro MUI_PAGE_WELCOME
; No directory page: the data directory is fixed for upgrade compatibility.
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_WELCOME
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH

!insertmacro MUI_LANGUAGE "English"

Section "Install"
  SetOutPath "$INSTDIR"
  SetOverwrite on

  ; Upgrade path: the install directory is shared with older Focusd/DeepFocus
  ; builds on purpose, so setup replaces binaries without touching user data.
  IfFileExists "$INSTDIR\uninstall.exe" 0 no_existing_install
    DetailPrint "Existing installation found: upgrading in place (data is kept)."
  no_existing_install:

  ; Stop the running tracker and GUI before overwriting binaries. Only
  ; processes launched from this install directory are stopped.
  DetailPrint "Stopping running DeepFocus processes..."
  IfFileExists "$INSTDIR\deepfocus.exe" 0 +3
    ExecWait '"$INSTDIR\deepfocus.exe" stop'
    Sleep 1000
  IfFileExists "$INSTDIR\focusd.exe" 0 +3
    ExecWait '"$INSTDIR\focusd.exe" stop'
    Sleep 500

  ; A plain powershell.exe is 32-bit under WOW64 and cannot read the image
  ; path of a 64-bit process; Sysnative selects the 64-bit PowerShell.
  StrCpy $PsExe "powershell.exe"
  IfFileExists "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe" 0 +2
    StrCpy $PsExe "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  System::Call 'kernel32::SetEnvironmentVariable(t "DEEPFOCUS_INSTDIR", t "$INSTDIR")'
  ExecWait `"$PsExe" -NoProfile -Command "Get-Process deepfocus_daemon, focusd_daemon -ErrorAction SilentlyContinue | Where-Object { $$_.Path -ne $$null -and $$_.Path.StartsWith($$env:DEEPFOCUS_INSTDIR + '\', [StringComparison]::OrdinalIgnoreCase) } | Stop-Process -Force"`
  ExecWait `"$PsExe" -NoProfile -Command "Get-Process deepfocus, deepfocus_gui, focusd, focusd_gui -ErrorAction SilentlyContinue | Where-Object { $$_.Path -ne $$null -and $$_.Path.StartsWith($$env:DEEPFOCUS_INSTDIR + '\', [StringComparison]::OrdinalIgnoreCase) } | Stop-Process -Force"`
  Sleep 500

  ; A stopped process can still hold its image for a moment. Retry the delete
  ; instead of letting File abort the whole upgrade.
  StrCpy $1 0
  retry_remove_old:
    Delete "$INSTDIR\deepfocus.exe"
    Delete "$INSTDIR\deepfocus_gui.exe"
    Delete "$INSTDIR\deepfocus_daemon.exe"
    IfFileExists "$INSTDIR\deepfocus_gui.exe" 0 old_files_removed
    IntOp $1 $1 + 1
    IntCmp $1 10 old_files_removed
    Sleep 500
    Goto retry_remove_old
  old_files_removed:

  File "deepfocus.exe"
  File "deepfocus_gui.exe"
  File "deepfocus_daemon.exe"

  ; Legacy names from Focusd. Removing them avoids two copies on PATH and an
  ; old daemon racing the new one.
  Delete "$INSTDIR\focusd.exe"
  Delete "$INSTDIR\focusd_gui.exe"
  Delete "$INSTDIR\focusd_daemon.exe"
  Delete "$DESKTOP\Focusd.lnk"
  Delete "$SMPROGRAMS\Focusd.lnk"
  Delete "$SMSTARTUP\Focus Daemon.lnk"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "focusd"
  DeleteRegKey HKCU "${LEGACY_UNINST_KEY}"

  CreateShortCut "$DESKTOP\DeepFocus.lnk" "$INSTDIR\deepfocus_gui.exe" "" "$INSTDIR\deepfocus_gui.exe" 0
  CreateShortCut "$SMPROGRAMS\DeepFocus.lnk" "$INSTDIR\deepfocus_gui.exe" "" "$INSTDIR\deepfocus_gui.exe" 0

  ; The registry app name must match system/registry.go (appName = DeepFocus).
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "DeepFocus" '"$INSTDIR\deepfocus_daemon.exe"'

  DetailPrint "Adding DeepFocus to your user PATH..."
  IfFileExists "$INSTDIR\deepfocus.exe" 0 skip_path_enable
    ExecWait '"$INSTDIR\deepfocus.exe" path enable' $0
    StrCmp $0 "0" +2
      DetailPrint "WARNING: could not add DeepFocus to PATH (exit code $0). Run 'deepfocus path enable' manually."
  skip_path_enable:

  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "DeepFocus Screen Time Tracker"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "COH2357"
  WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout" "https://github.com/COH2357/DeepFocus"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" '"$INSTDIR\deepfocus_gui.exe"'

  WriteUninstaller "$INSTDIR\uninstall.exe"

  DetailPrint "Starting DeepFocus tracker..."
  ExecShell "open" "$INSTDIR\deepfocus_daemon.exe" "" SW_HIDE
  ; Silent upgrades should not pop a window; the user is probably not present.
  IfSilent skip_gui_launch
  ExecShell "open" "$INSTDIR\deepfocus_gui.exe"
  skip_gui_launch:
SectionEnd

Section "Uninstall"
  DetailPrint "Stopping DeepFocus processes..."
  IfFileExists "$INSTDIR\deepfocus.exe" 0 +3
    ExecWait '"$INSTDIR\deepfocus.exe" stop'
    Sleep 1000
  IfFileExists "$INSTDIR\focusd.exe" 0 +3
    ExecWait '"$INSTDIR\focusd.exe" stop'
    Sleep 500

  StrCpy $PsExe "powershell.exe"
  IfFileExists "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe" 0 +2
    StrCpy $PsExe "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  System::Call 'kernel32::SetEnvironmentVariable(t "DEEPFOCUS_INSTDIR", t "$INSTDIR")'
  ExecWait `"$PsExe" -NoProfile -Command "Get-Process deepfocus_daemon, focusd_daemon -ErrorAction SilentlyContinue | Where-Object { $$_.Path -ne $$null -and $$_.Path.StartsWith($$env:DEEPFOCUS_INSTDIR + '\', [StringComparison]::OrdinalIgnoreCase) } | Stop-Process -Force"`
  ExecWait `"$PsExe" -NoProfile -Command "Get-Process deepfocus, deepfocus_gui, focusd, focusd_gui -ErrorAction SilentlyContinue | Where-Object { $$_.Path -ne $$null -and $$_.Path.StartsWith($$env:DEEPFOCUS_INSTDIR + '\', [StringComparison]::OrdinalIgnoreCase) } | Stop-Process -Force"`
  Sleep 500

  ; Remove PATH entries while the directories still exist.
  IfFileExists "$INSTDIR\deepfocus.exe" 0 skip_path_disable
    ExecWait '"$INSTDIR\deepfocus.exe" path disable' $0
    StrCmp $0 "0" +2
      DetailPrint "WARNING: could not remove DeepFocus from PATH (exit code $0). Run 'deepfocus path disable' manually."
  skip_path_disable:

  Delete "$INSTDIR\deepfocus.exe"
  Delete "$INSTDIR\deepfocus_gui.exe"
  Delete "$INSTDIR\deepfocus_daemon.exe"
  Delete "$INSTDIR\focusd.exe"
  Delete "$INSTDIR\focusd_gui.exe"
  Delete "$INSTDIR\focusd_daemon.exe"
  Delete "$DESKTOP\DeepFocus.lnk"
  Delete "$SMPROGRAMS\DeepFocus.lnk"
  Delete "$DESKTOP\Focusd.lnk"
  Delete "$SMPROGRAMS\Focusd.lnk"
  Delete "$SMSTARTUP\Focus Daemon.lnk"
  Delete "$INSTDIR\deepfocus_start.vbs"
  Delete "$INSTDIR\focusd_start.vbs"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "DeepFocus"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "focusd"
  Delete "$INSTDIR\uninstall.exe"

  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "${LEGACY_UNINST_KEY}"

  ; $INSTDIR also holds focusd.db and config.json: the user's history. Keep it.
  RMDir "$INSTDIR"
SectionEnd
