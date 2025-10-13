// stub CloudFlare worker script which is used to power the badges
addEventListener("fetch", event => {
  event.respondWith(handleRequest(event.request))
})

async function handleRequest(request) {
  var path  = new URL(request.url.toString()).pathname
  var route = path.match(/^\/(?<collection>[a-z_]+?)(?:_(?<type>public|private))?\/(?<login>[^\/\.]+)(\.(?<extension>(svg)))?$/)
  
  if (route == null) {
    if (path == "/") {
      return Response.redirect(BASE_URL+"/#badges")
    } else {
      return new Response("Invalid URL requested: "+path, { status: 400 })
    }
  }
  
  var collectionKey = route.groups["collection"] + (route.groups["type"] ? "_" + route.groups["type"] : "")

  if (collectionKey in DATA) {
    if (typeof route.groups["extension"] === "undefined") {
      return Response.redirect(BASE_URL + "/" + route.groups["collection"] + "#" + route.groups["login"])
    } else {
      var rank = 1 + DATA[route.groups["collection"]].indexOf(route.groups["login"])
      var displayName = (typeof TITLES !== "undefined" && TITLES[collectionRaw]) ? TITLES[collectionRaw] : ""
      
      var color = rank == 0 ? "red" : "brightgreen"

      var collectionRaw = route.groups["collection"];

      // descriptor lookup from captured type
      const DESCRIPTOR = { default: "public commits", public: "public contributions", private: "all contributions" }
      var descriptor = DESCRIPTOR[route.groups["type"] || "default"]

      // right-hand message: "#N <DisplayName> (<descriptor>)" or "unranked <DisplayName> (<descriptor>)"
      var message = (rank == 0 ? "unranked " : "#" + rank + " ") + displayName + " (" + descriptor + ")"

      var label = "committers.top rank"

      var shieldsUrl = "https://img.shields.io/badge/" + encodeURIComponent(label) + "-" + encodeURIComponent(message) + "-" + encodeURIComponent(color)
     
      var shieldReq = new Request(shieldsUrl)
      var response = await fetch(shieldReq)
      var result = new Response(response.body, response)
      result.headers.set('Cache-Control', 'private, max-age=600, must-revalidate')
     
      return result
    }
  } else {
    return new Response("Country/Region not found: "+route.groups["collection"], { status: 404 })
  }
}

// BASE_URL and DATA will be defined during deployment process
