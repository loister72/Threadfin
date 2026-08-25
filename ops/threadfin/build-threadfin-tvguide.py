#!/usr/bin/env python3
import re, shutil, time, os
import xml.etree.ElementTree as ET
from pathlib import Path

M3U_PATH=Path("/opt/threadfin/conf/provider/livingroom-us-skinny-75075.m3u")
OUT_PATH=Path("/opt/threadfin/conf/provider/livingroom-us-skinny-75075.xml")
TV_PATH=Path("/opt/threadfin/tvguide/directv-dallas.xml")

manual={
"KDFW FOX 4 Dallas":"19616","KXAS NBC 5 Fort Worth":"19627","WFAA ABC 8 Dallas-Fort Worth":"19626","KTVT CBS 11 Fort Worth":"20204","PBS":"20372","KTXA 21 Dallas":"25153",
"CNN":"58646","HLN":"64549","Fox News Channel":"60179","MSNBC":"64241","CNBC":"58780","CNBC World":"26849","Fox Business Network":"58718","Bloomberg Television":"71799","BBC News":"101449","The Weather Channel":"58812","AccuWeather":"91994","Newsmax":"97163",
"ESPN":"32645","ESPN2":"45507","ESPNews":"61812","ESPNU":"60696","FS1":"82547","FS2":"59305","CBS Sports Network":"59250","NFL Network":"45399","NFL RedZone":"65025","MLB Network":"62081","MLB Strike Zone":"75220","NBA TV":"45526","NHL Network":"58690","Golf Channel":"61854","Tennis Channel":"60316","SEC Network":"89714","ACC Network":"111871","Big Ten Network":"58321","Longhorn Network":"94102","FanDuel Sports Southwest":"193821","Space City Home Network":"193822","beIN Sports":"61143","TUDN":"77033","ESPN Deportes":"71914","Fox Deportes":"72189","MAVTV":"46737","MotorTrend":"31046","Outdoor Channel":"46737","Sportsman Channel":"77423","Cowboy Channel":"63717",
"USA Network":"58452","TNT":"42642","TBS":"58515","FX":"58574","FXX":"66379","Paramount Network":"59186","Comedy Central":"62420","AMC":"59337","Syfy":"58623","BBC America":"64492","A&E":"51529","Bravo":"58625","WE tv":"59296","Oxygen True Crime":"70522","REELZ":"68385","TV Land":"73541","Freeform":"59615","BET":"63236","BET Her":"63236","MTV":"60964","MTV2":"75077","MTV Classic":"64630","MTV Live":"49141","VH1":"60046","CMT":"59440","IFC":"59444","SundanceTV":"71280","WGN America":"91096","Pop":"68796","VICE":"65732","Logo":"110289","UPtv":"82892","INSP":"82773","ION":"76894","AXS TV":"28506","Revolt":"83098","Cleo TV":"110289","TV One":"61960","Fuse":"59116","Aspire":"97409","Lifetime":"60150","truTV":"64490","FYI":"58988","Ovation":"69061",
"HGTV":"49788","Food Network":"50747","Cooking Channel":"68065","TLC":"57391","Discovery":"56905","Discovery Channel":"56905","Science Channel":"57390","History":"57708","Military History":"78808","Smithsonian Channel":"58532","National Geographic":"49438","Animal Planet":"57394","Travel Channel":"59303","Investigation Discovery":"65342","Destination America":"60468","Discovery Life":"19247","OWN":"70388","Game Show Network":"68827","Tastemade":"107076","AWE":"126128","Outside TV":"46737","Dog TV":"82446","RFD-TV":"63717",
"HBO":"19548","HBO2":"59368","HBO Signature":"59363","HBO Comedy":"59839","HBO Family":"59355","HBO Zone":"59845","HBO Latino":"59837","Cinemax":"34933","MoreMAX":"35975","ActionMAX":"59948","5StarMAX":"59961","MovieMAX":"59373","OuterMAX":"59965","Paramount+ with SHOWTIME":"21868","Showtime 2":"58533","Showtime Extreme":"60947","Showtime Showcase":"61001","Showtime Next":"68342","Showtime Family Zone":"103892","STARZ":"34941","STARZ Edge":"57581","STARZ Comedy":"57569","STARZ In Black":"67235","STARZ Encore":"36225","STARZ Encore Action":"72015","STARZ Encore Black":"67236","STARZ Encore Classic":"57573","STARZ Encore Suspense":"57569","STARZ Encore Westerns":"36225","EPIX":"65687","EPIX 2":"67929","EPIX Hits":"74073","Hallmark Channel":"66268","Hallmark Mystery":"46710","Lifetime Movie Network":"55887","TCM":"64312","FX Movie Channel":"70253","Sony Movies":"69130","HDNet Movies":"33668","PixL":"50404","AMC+":"114759","STARZ Cinema":"67236",
"Cartoon Network":"60048","Adult Swim":"60048","Nickelodeon":"59432","Nick Jr.":"82649","Nicktoons":"59432","Disney Channel":"59684","Disney XD":"60006","Disney Junior":"67749","Boomerang":"159817","Discovery Family":"67749","Cartoonito":"159817","Univision":"35969","UniMás":"27421","Telemundo":"20773","Universo":"91588","Galavision":"68367","QVC":"60222","QVC2":"60222","HSN":"62077","TBN":"59539","MeTV":"70436","Court TV":"109454","Comet":"73067","Antenna TV":"70436","Bounce TV":"73067","Grit":"185303"
}

logo_overrides={
    "KXAS NBC 5 Fort Worth":"https://logo.m3uassets.com/nbc.png",
    "FanDuel Sports Southwest":"https://logo.m3uassets.com/ballysportssouthwest.png",
    "UPtv":"https://logo.m3uassets.com/uptv.png",
}

m3u=M3U_PATH.read_text(errors="ignore")
rewritten_m3u=[]
current=[]
for line in m3u.splitlines():
    if line.startswith("#EXTINF"):
        tvgid=re.search(r"tvg-id=\"([^\"]*)\"", line).group(1)
        chno=re.search(r"tvg-chno=\"([^\"]*)\"", line).group(1)
        logo=(re.search(r"tvg-logo=\"([^\"]*)\"", line) or [None,""])[1]
        name=line.rsplit(",",1)[-1]
        if name in logo_overrides:
            logo=logo_overrides[name]
            line=re.sub(r'tvg-logo="[^"]*"', f'tvg-logo="{logo}"', line)
        current.append((tvgid,chno,name,logo))
    rewritten_m3u.append(line)

new_m3u="\n".join(rewritten_m3u)+"\n"
if new_m3u != m3u:
    shutil.copy2(M3U_PATH, M3U_PATH.with_suffix(M3U_PATH.suffix+f".bak-{int(time.time())}"))
    M3U_PATH.write_text(new_m3u)

tv=ET.parse(TV_PATH).getroot()
tv_channels={c.attrib.get("id"):c for c in tv.findall("channel")}
programmes_by_channel={}
for p in tv.findall("programme"):
    programmes_by_channel.setdefault(p.attrib.get("channel"), []).append(p)

def plex_safe_programme(programme, channel_id):
    cp=ET.fromstring(ET.tostring(programme, encoding="utf-8"))
    cp.attrib["channel"]=channel_id

    # Plex can render XMLTV episode rows with blank episode titles as
    # "Unknown Airing". Flatten provider metadata so the programme title is
    # always the visible guide title.
    for tag in ("episode-num", "sub-title", "new", "previously-shown"):
        for child in list(cp.findall(tag)):
            cp.remove(child)

    title=cp.find("title")
    if title is None:
        title=ET.Element("title")
        cp.insert(0, title)
    if not (title.text or "").strip():
        fallback=programme.findtext("sub-title") or "Unknown Airing"
        title.text=fallback.strip()

    return cp

if OUT_PATH.exists():
    shutil.copy2(OUT_PATH, OUT_PATH.with_suffix(OUT_PATH.suffix+f".bak-{int(time.time())}"))

new=ET.Element("tv")
matched=programmes=0
programme_copies=[]
for tvgid,chno,name,logo in current:
    logo=logo_overrides.get(name, logo)
    ch=ET.SubElement(new,"channel",{"id":tvgid})
    ET.SubElement(ch,"display-name").text=name
    ET.SubElement(ch,"display-name").text=chno
    if logo:
        ET.SubElement(ch,"icon",{"src":logo})
    src=manual.get(name)
    if src in tv_channels:
        matched+=1
        for p in programmes_by_channel.get(src,[]):
            programme_copies.append(plex_safe_programme(p, tvgid))
            programmes+=1

programme_copies.sort(key=lambda p: (
    p.attrib.get("channel",""),
    p.attrib.get("start",""),
    p.attrib.get("stop",""),
    p.findtext("title") or "",
))
for p in programme_copies:
    new.append(p)

ET.indent(new, space="  ")
tmp=OUT_PATH.with_suffix(".tmp")
ET.ElementTree(new).write(tmp, encoding="utf-8", xml_declaration=True)
os.replace(tmp, OUT_PATH)
print(f"matched={matched} channels={len(current)} programmes={programmes}")
