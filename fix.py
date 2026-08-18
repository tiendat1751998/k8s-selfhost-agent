import re

# Fix backup_handler.go
path = r'd:\project\k8sseflhost\internal\adapter\http\backup_handler.go'
with open(path, 'r') as f:
    content = f.read()

content = re.sub(r'json:"(r²²"rrÂ&v§6öó¢%Ã"vÂ6öçFVçB§v—F‚÷Vâ‡F‚Âwrr’2c ¢bçw&—FR†6öçFVçB ¢2f—‚&6·W÷&Wòævğ§Fƒ"Ò"vC¥Ç&ö¦V7EÆ³‡76VfÆ†÷7EÆ–çFW&æÅÆ–æg&7G'V7GW&UÇ÷7Fw&W5Æ&6·W÷&Wòævòp§v—F‚÷Vâ‡Fƒ"Âw"r’2c ¢6öçFVçC"Òbç&VB‚ ¦6öçFVçC"Ò6öçFVçC"ç&WÆ6R‚wVW'’£ÒÆåÇEÇD”å4U%BrÂwVW'’£ÒÆåÇEÇD”å4U%Br¦6öçFVçC"Ò6öçFVçC"ç&WÆ6R‚u$UEU$ä”är–BrÂu$UEU$ä”är–Fr¦6öçFVçC"Ò6öçFVçC"ç&WÆ6R‚wVW'’£Ò4TÄT5B–BrÂwVW'’£Ò4TÄT5B–Br¦6öçFVçC"Ò6öçFVçC"ç&WÆ6R‚wFVæçEö–BÒCrÂwFVæçEö–BÒCr¦6öçFVçC"Ò6öçFVçC"ç&WÆ6R‚v–BÒCrÂv–BÒCr §v—F‚÷Vâ‡Fƒ"Âwrr’2c ¢bçw&—FR†6öçFVçC" 