// Prints out a Go array with demon names (no duplicates).
// The order of the names is always the same for a given number of names.
// The fixed names are real demons from Wikipedia.
// The random generator is translated to Go from 
// http://fantasynamegenerators.com/scripts/demonNames.js
// In most jurisdictions neither the fixed list nor the code will satisfy the
// criteria for copyright protection.
// Treat them as public domain. No one will sue you.

package main

import "math/rand"
import "strings"
import "fmt"

const NUM_DEMONS = 65536

var fixed = []string{"Aamon","Abaddon","Abalam","Abezethibou","Abraxas","Abyzou","Ad","Adramelech","Aeshma","Agaliarept","Agares","Agiel","Ahriman","Aim","Akem","Ala","Alal","Alastor","Alloces","Allu","Amaymon","Amdusias","Ammut","Amon","Amy","Anamalech","Andhaka","Andras","Andrealphus","Andromalius","Antichrist","Anzu","Apep","Apollyon","Armaros","Asag","Asb","Asmodai","Astaroth","Astarte","Asura","Azazel","Azi","Baal","Balam","Balberith","Bali","Banshee","Baphomet","Barbas","Barbatos","Bathin","Beelzebub","Behemoth","Beherit","Beleth","Belial","Belphegor","Berith","Bh","Bies","Bifrons","Boruta","Botis","Buer","Bukavac","Bune","Bushyasta","Caacrinolaas","Caim","Charun","Chemosh","Cimejes","Corson","Crocell","Crone","Culsu","Daeva","Dagon","Danjal","Dantalion","Dasa","Davy","Decarabia","Demogorgon","Devil","Dhajjal","Div","Donn","Drekavac","Dumah","Eligos","Empusa","Euryale","Eurynome","Eurynomos","Familiars","Focalor","Foras","Forneus","Furcas","Furfur","Furies","Gader","Gaki","Gello","Glasya","Gorgon","Gremory","Grigori","Gualichu","Gusion","Haagenti","Halphas","Haures","Humbaba","Iblis","Ifrit","Incubus","Ipos","Jikininki","Jinn","Kabandha","Kitsune","Kokb","Labal","Lady","Lamashtu","Lamia","Lechies","Lempo","Leraje","Leviathan","Leyak","Lilim","Lilin","Lilith","Lix","Lucifer","Lucifuge","Malphas","Mammon","Mara","Marax","Marbas","Marchosias","Maricha","Mastema","Mathim","Medusa","Mephistopheles","Merihem","Mictlantecuhtli","Moloch","Murmur","Naamah","Naberius","Naberus","Naphula","Nekomata","Neqa","Ninurta","Nisroch","Nix","Nyai","Obizoth","Oni","Onoskelis","Oray","Orcus","Oriax","Orobas","Ose","Paimon","Pazuzu","Penemue","Phenex","Pithius","Pocong","Pontianak","Popobawa","Procell","Pruflas","Psoglav","Purson","Putana","Rahab","Rahovart","Raiju","Rakshasa","Rangda","Raum","Ravana","Ronove","Rosier","Rumjal","Rusalka","Sabnock","Saiko","Sallos","Salpsan","Samael","Satan","Satanachia","Scox","Seere","Semyazza","Set","Shaitan","Shax","Shedim","Shezmu","Sidragasum","Sitri","Stheno","Stolas","Stuha","Succubus","Surgat","Tannin","Tartaruchi","Teeraal","Temeluchus","Tengu","Titivillus","Tuyul","Ukobach","Utukku","Valefar","Vapula","Vassago","Vepar","Verrine","Vine","Volac","Vual","Vucub","Wekufe","Wendigo","Xaphan","Yeqon","Yeter","Yokai","Yuki","Zaebos","Zagan","Zalambur","Zepar","Zin","Ziz","Zmeu"}
var names = []string{}
var have = map[string]bool{}

func nameGen() []string {
  var characters1 = []string{"baal","bal","bael","bar","barb","bas","bat","beal","beb","beel","beh","bel","ber","bil","bin","bit","bof","bol","bot","bun","bul","cam","car","caym","cer","char","chax","cher","cim","cor","cul","cur","dan","djin","dar","fam","foc","for","fur","far","fhar","fhur","fhor","fham","gad","gak","gam","gaem","gob","gom","gin","gor","grem","gus","guz","hab","hal","han","hav","haur ","hir","hum","jik","jin","kas","kim","kok","kos","lab","lam","lem","ler","ler","lil","mal","mam","mar","mas","mat","med","mel","mep","mer","mol","mor","mur","nab","nap","neq","nin","pay","paz","per","phen","pin","pir","pit","pul","pur","qen","rab","rah","raim","raum","ron","ron","rum","rum","rus","sab","sal","sam","sear","seir","sem","sep","shax","shed","sid","sip","sit","stol","sur","syd","tan","tap","ten","tham","tip","ton","tum","tur","vad","val","van","vap","vas","ve","vep","vep","vin","vol","vos","vual","wal","xez","xer","xar","xaz","xaer","zag","zaeb","zep","zim","zar","zam","zaem"}
  var characters2 = []string{"a","e","o","u"}
  var characters3 = []string{"ba","bae","bai","bao","bau","be","bea","bei","beo","beu","bi","bia","bie","bio","biu","bo","boa","boi","bou","bu","bua","bue","bui","buo","ca","cae","cai","cao","cau","ce","cea","cei","ceo","ceu","ci","cia","cio","ciu","co","coa","coi","cou","cu","cua","cue","cui","cuo","da","dae","dai","dao","dau","de","dea","dei","deo","deu","di","dia","dio","diu","do","doa","doi","dou","du","dua","due","dui","duo","fa","fae","fai","fao","fau","fe","fea","fei","feo","feu","fi","fia","fie","fio","fiu","fo","foa","foi","fou","fu","fua","fue","fui","fuo","ga","gae","gai","gao","gau","ge","gea","gei","geo","geu","gi","gia","gie","gio","giu","go","goa","goi","gou","gu","gua","gue","gui","guo","ha","hae","hai","hao","hau","he","hea","hei","heo","heu","hi","hia","hie","hio","hiu","ho","hoa","hoi","hou","hu","hua","hue","hui","huo","ka","kae","kai","kao","kau","ke","kea","kei","keo","keu","ki","kia","kie","kio","kiu","ko","koa","koi","kou","ku","kua","kue","kui","kuo","la","lae","lai","lao","lau","le","lea","lei","leo","leu","li","lia","lie","lio","liu","lo","loa","loi","lou","lu","lua","lue","lui","luo","ma","mae","mai","mao","mau","me","mea","mei","meo","meu","mi","mia","mie","mio","miu","mo","moa","moi","mou","mu","mua","mue","mui","muo","na","nae","nai","nao","nau","ne","nea","nei","neo","neu","ni","nia","nie","nio","niu","no","noa","noi","nou","nu","nua","nue","nui","nuo","pa","pae","pai","pao","pau","pe","pea","pei","peo","peu","pi","pia","pie","pio","piu","po","poa","poi","pou","pu","pua","pue","pui","puo","qa","qae","qai","qao","qau","qe","qea","qei","qeo","qeu","qi","qia","qie","qio","qiu","qo","qoa","qoi","qou","qu","qua","que","qui","quo","ra","rae","rai","rao","rau","re","rea","rei","reo","reu","ri","ria","rie","rio","riu","ro","roa","roi","rou","ru","rua","rue","rui","ruo","sa","sae","sai","sao","sau","se","sea","sei","seo","seu","si","sia","sie","sio","siu","so","soa","soi","sou","su","sua","sue","sui","suo","ta","tae","tai","tao","tau","te","tea","tei","teo","teu","ti","tia","tie","tio","tiu","to","toa","toi","tou","tu","tua","tue","tui","tuo","va","vae","vai","vao","vau","ve","vea","vei","veo","veu","vi","via","vie","vio","viu","vo","voa","voi","vou","vu","vua","vue","vui","vuo","wa","wae","wai","wao","wau","we","wea","wei","weo","weu","wi","wia","wie","wio","wiu","wo","woa","woi","wou","wu","wua","wue","wui","wuo","xa","xae","xai","xao","xau","xe","xea","xei","xeo","xeu","xi","xia","xie","xio","xiu","xo","xoa","xoi","xou","xu","xua","xue","xui","xuo","za","zae","zai","zao","zau","ze","zea","zei","zeo","zeu","zi","zia","zie","zio","ziu","zo","zoa","zoi","zou","zu","zua","zue","zui","zuo"}
  var characters4 = []string{"a","aa","ae","au","ao","ai","e","ee","ea","ei","eo","eu","i","ia","io","iu","ie","o","oo","ou","oa","oe","oi","u","uu","ua","ue","ui","uo"}
  var characters5 = []string{"rch","ch","r","hs","sum","som","sam","sahm","sohm","suhm","sir","sihr","sohr","sor","sur","suhr","sach","rach","roch","rus","rum","ram","rom","rohm","rahm","ruhm","run","ran","ron","rin","rihn","rohn","rohm","rahn","buhr","bohr","bihr","bach","bal","bahl","boch","bahr","bur","bor","bar","buhn","buhm","bun","bohn","bon","duhr","dohr","dal","dahl","dihr","dach","doch","dahr","dur","dor","dar","duhn","duhm","dun","dohn","don","muhr","mohr","mihr","mach","moch","mahr","mur","mor","mal","mahl","mar","muhn","muhm","mun","mohn","mon","nuhr","nohr","nihr","nach","noch","nahr","nur","nor","nar","nuhn","nuhn","nun","nohn","non","xuhr","xohr","xihr","xach","xoch","xahr","xur","xor","xar","xuhn","xuhm","xun","xohn","xon","zuhr","zohr","zihr","zach","zoch","zahr","zur","zor","zar","zuhn","zuhm","zun","zohn","zon","zal","zahl","xal","xahl","zhal","zhor","zhar","bhal","dhal","xhar","xhal","luhr","lohr","lihr","lach","loch","lahr","lur","lor","lar","luhn","luhm","lun","lohn","lon","lhar","lhan","lham"}
  var characters6 = []string{"br","cr","dr","fr","gr","pr","str","tr","bl","cl","fl","gl","pl","sl","sc","sk","sm","sn","sp","st","sw","ch","sh","th","wh"}
				
  var random1 = rand.Intn(len(characters1))
  var random2 = rand.Intn(len(characters2))
  var random3 = rand.Intn(len(characters5))

  var random4 = rand.Intn(len(characters2))
  var random5 = rand.Intn(len(characters3))
  var random6 = rand.Intn(len(characters5))

  var random7 = rand.Intn(len(characters6))
  var random8 = rand.Intn(len(characters2))
  var random9 = rand.Intn(len(characters3))
  var random10 = rand.Intn(len(characters5))	

  var random11 = rand.Intn(len(characters4))
  var random12 = rand.Intn(len(characters2))
  var random13 = rand.Intn(len(characters5))
                          
  var random14 = rand.Intn(len(characters1))
  var random15 = rand.Intn(len(characters2))
  var random16 = rand.Intn(len(characters3))
  var random17 = rand.Intn(len(characters5))	

  var random18 = rand.Intn(len(characters2))
  var random19 = rand.Intn(len(characters6))
  var random20 = rand.Intn(len(characters2))
  var random21 = rand.Intn(len(characters5))	
                  
  var random22 = rand.Intn(len(characters2))
  var random23 = rand.Intn(len(characters1))
  var random24 = rand.Intn(len(characters2))
  var random25 = rand.Intn(len(characters5))			
          
  var random26 = rand.Intn(len(characters6))
  var random27 = rand.Intn(len(characters4))
  var random28 = rand.Intn(len(characters5))
  var random29 = rand.Intn(len(characters2))	
                  
  var random30 = rand.Intn(len(characters1))
  var random31 = rand.Intn(len(characters4))
  var random32 = rand.Intn(len(characters6))
  var random33 = rand.Intn(len(characters4))	
                          
  var random34 = rand.Intn(len(characters4))
  var random35 = rand.Intn(len(characters3))
  var random36 = rand.Intn(len(characters5))			

  var name = strings.ToUpper(characters1[random1][0:1]) + characters2[random2] + characters5[random3];
  var name2 = strings.ToUpper(characters2[random4][0:1]) + characters3[random5] + characters5[random6];	
  var name3 = strings.ToUpper(characters6[random7][0:1]) + characters2[random8] + characters3[random9] + characters5[random10];
  var name4 = strings.ToUpper(characters4[random11][0:1]) + characters2[random12] + characters5[random13];
  var name5 = strings.ToUpper(characters1[random14][0:1]) + characters2[random15] + characters3[random16] + characters5[random17];	
  var name6 = strings.ToUpper(characters2[random18][0:1]) + characters6[random19] + characters2[random20] + characters5[random21];
  var name7 = strings.ToUpper(characters2[random22][0:1]) + characters1[random23] + characters2[random24] + characters5[random25];
  var name8 = strings.ToUpper(characters6[random26][0:1]) + characters4[random27] + characters5[random28] + characters2[random29];
  var name9 = strings.ToUpper(characters1[random30][0:1]) + characters4[random31] + characters6[random32] + characters4[random33];	
  var name10 = strings.ToUpper(characters4[random34][0:1]) + characters3[random35] + characters5[random36];
  
  return []string{name,name2,name3,name4,name5,name6,name7,name8,name9,name10}
}

func main() {
  for _,name := range fixed {
    have[name] = true
  }
  
  for len(names)+len(fixed) < NUM_DEMONS {
    n := nameGen()
    for i := range n {
      if ! have[n[i]] {
        names = append(names, n[i])
        have[n[i]] = true
      }
    }
  }

  result := []string{}
  for _,i := range rand.Perm(len(fixed)) {
    result = append(result, fixed[i])
  }
  
  for _,i := range rand.Perm(len(names)) {
    if len(result) == NUM_DEMONS { break }
    result = append(result, names[i])
  }
  
  fmt.Print("var DEMONS = []string{")
  for i := 0; i < NUM_DEMONS; i++ {
    if i > 0 { fmt.Print(", ") }
    fmt.Printf("\"%s\"",result[i])
  }
  fmt.Println("}")
}
