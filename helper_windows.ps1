$OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

Add-Type -AssemblyName System.Windows.Forms

$source = [System.Windows.Forms.InputLanguage]::CurrentInputLanguage
$im = $source.LayoutName

$caps = [System.Windows.Forms.Control]::IsKeyLocked('CapsLock')

[Console]::Out.Write("$im|$caps")
