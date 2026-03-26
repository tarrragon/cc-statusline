Add-Type -AssemblyName System.Windows.Forms

$source = [System.Windows.Forms.InputLanguage]::CurrentInputLanguage
$im = $source.LayoutName

$caps = [System.Windows.Forms.Control]::IsKeyLocked('CapsLock')

Write-Host "$im|$caps"
