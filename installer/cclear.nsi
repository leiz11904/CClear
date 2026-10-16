; ???????? scripts/build-windows.sh ??????????
Unicode true
ManifestDPIAware true

!include "MUI2.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!include "FileFunc.nsh"
!include "WinMessages.nsh"
!include "LogicLib.nsh"

!define PRODUCT_NAME "CClear"
!define PRODUCT_VERSION "3.0.0"
!define PRODUCT_PUBLISHER "CClear"
!define APP_EXE "cclear.exe"

Name "${PRODUCT_NAME}"
Caption "CClear 安装程序"
OutFile "..\dist\CClear-Setup-${PRODUCT_VERSION}.exe"
InstallDir "$PROGRAMFILES64\CClear"
InstallDirRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "InstallLocation"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "CClear ${PRODUCT_VERSION}"
ShowInstDetails show
ShowUninstDetails show

Icon "..\assets\cclear-v3.ico"
UninstallIcon "..\assets\cclear-v3.ico"

VIProductVersion "3.0.0.0"
VIFileVersion "3.0.0.0"
VIAddVersionKey /LANG=2052 "ProductName" "CClear"
VIAddVersionKey /LANG=2052 "CompanyName" "CClear"
VIAddVersionKey /LANG=2052 "FileDescription" "CClear 安装程序"
VIAddVersionKey /LANG=2052 "FileVersion" "${PRODUCT_VERSION}"
VIAddVersionKey /LANG=2052 "ProductVersion" "${PRODUCT_VERSION}"
VIAddVersionKey /LANG=2052 "LegalCopyright" "Copyright (C) 2026 CClear"
VIAddVersionKey /LANG=2052 "OriginalFilename" "CClear-Setup-${PRODUCT_VERSION}.exe"

!define MUI_ABORTWARNING
!define MUI_ICON "..\assets\cclear-v3.ico"
!define MUI_UNICON "..\assets\cclear-v3.ico"
!define MUI_WELCOMEPAGE_TITLE "欢迎使用 CClear 安装向导"
!define MUI_WELCOMEPAGE_TEXT "此向导将把 CClear 安装到你的计算机。$\r$\n$\r$\nCClear 用于安全清理系统盘上的临时文件、缓存和回收站，以释放磁盘空间。$\r$\n$\r$\n它不会删除文档、桌面、下载目录，也不会卸载已安装的软件。清理前可以先扫描并查看将要删除的内容。$\r$\n$\r$\n点击“下一步”继续。"
!define MUI_LICENSEPAGE_TEXT_TOP "请阅读下列说明。安装后可以从 Windows 的“应用和功能”中卸载 CClear。"
!define MUI_DIRECTORYPAGE_TEXT_TOP "安装程序将把 CClear 安装到下列文件夹。$\r$\n$\r$\n要安装到其他文件夹，请点击“浏览”并选择其他文件夹。"
!define MUI_FINISHPAGE_TITLE "安装完成"
!define MUI_FINISHPAGE_TEXT "CClear 已经安装到你的计算机。$\r$\n$\r$\n建议先扫描，确认列表后再清理。带警告的项目默认不会勾选。"
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "启动 CClear"
!define MUI_FINISHPAGE_RUN_NOTCHECKED

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "license.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "CClear 需要 64 位 Windows。"
    Abort
  ${EndIf}
  ${If} ${AtMostWin8.1}
    MessageBox MB_OK|MB_ICONSTOP "CClear 需要 Windows 10 或更高版本。"
    Abort
  ${EndIf}
  Call CloseApp
FunctionEnd

Function un.onInit
  Call un.CloseApp
FunctionEnd

Function CloseApp
  FindWindow $0 "CClearMainWindow" ""
  ${If} $0 <> 0
    MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "CClear 正在运行。$\n$\n点击“确定”关闭它并继续安装。" IDOK closeit
    Abort
    closeit:
      SendMessage $0 ${WM_CLOSE} 0 0
      Sleep 800
  ${EndIf}
FunctionEnd

Function un.CloseApp
  FindWindow $0 "CClearMainWindow" ""
  ${If} $0 <> 0
    MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "CClear 正在运行。$\n$\n点击“确定”关闭它并继续卸载。" IDOK closeit
    Abort
    closeit:
      SendMessage $0 ${WM_CLOSE} 0 0
      Sleep 800
  ${EndIf}
FunctionEnd

Section "CClear" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "..\dist\cclear.exe"
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  CreateDirectory "$SMPROGRAMS\CClear"
  CreateShortcut "$SMPROGRAMS\CClear\CClear.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0
  CreateShortcut "$SMPROGRAMS\CClear\卸载 CClear.lnk" "$INSTDIR\Uninstall.exe"
  CreateShortcut "$DESKTOP\CClear.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0

  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "DisplayName" "CClear C盘安全清理"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "DisplayIcon" "$INSTDIR\${APP_EXE}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear" "EstimatedSize" $0
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\CClear\CClear.lnk"
  Delete "$SMPROGRAMS\CClear\卸载 CClear.lnk"
  RMDir "$SMPROGRAMS\CClear"
  Delete "$DESKTOP\CClear.lnk"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CClear"
SectionEnd
