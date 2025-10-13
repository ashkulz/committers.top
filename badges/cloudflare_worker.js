// stub CloudFlare worker script which is used to power the badges
addEventListener("fetch", event => {
  event.respondWith(handleRequest(event.request))
})

async function handleRequest(request) {
  var path  = new URL(request.url.toString()).pathname
  var route = path.match(/^\/(?<collection>[a-z_]+)\/(?<login>[^\/\.]+)(\.(?<extension>(svg)))?$/)
  if (route == null) {
    if (path == "/") {
      return Response.redirect(BASE_URL+"/#badges")
    } else {
      return new Response("Invalid URL requested: "+path, { status: 400 })
    }
  }
  if (route.groups["collection"] in DATA) {
    var collection = DATA[route.groups["collection"]]
    if (typeof route.groups["extension"] === "undefined") {
      return Response.redirect(BASE_URL + "/" + route.groups["collection"] + "#" + route.groups["login"])
    } else {
      var rank = 1 + DATA[route.groups["collection"]].indexOf(route.groups["login"])
      
      var color = rank == 0 ? "red" : "brightgreen"

      var collectionRaw = route.groups["collection"];
      var parts = collectionRaw.split("_");
      var baseParts = parts.slice();

      // mapping rules:
      // - no suffix (single token) => "public commits"
      // - _public => "public contributions"
      // - _private => "all contributions"
      var descriptor = "public commits";

      if (parts.length > 1) {
        var suffix = parts[parts.length - 1];
        baseParts = parts.slice(0, parts.length - 1);
      
        if (suffix === "public") {
          descriptor = "public contributions";
        } else if (suffix === "private") {
          descriptor = "all contributions";
        }
      }

      // title-case the base parts and join with spaces
      for (var i = 0; i < baseParts.length; i++) {
        if (baseParts[i].length > 0) {
          baseParts[i] = baseParts[i].charAt(0).toUpperCase() + baseParts[i].slice(1)
        }
      }

      var displayName = baseParts.join(" ")

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
