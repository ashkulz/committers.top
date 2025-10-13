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
      return Response.redirect(BASE_URL+"/"+route.groups["collection"]+"#"+route.groups["login"])
    } else {
      var rank = 1 + DATA[route.groups["collection"]].indexOf(route.groups["login"])
      var badge = "committers.top rank-"+(rank == 0 ? "unranked-red" : "#"+rank+"-brightgreen")
      var request = new Request("https://img.shields.io/badge/"+encodeURIComponent(badge))
      var response = await fetch(request)
      var result = new Response(response.body, response)
      result.headers.set('Cache-Control', 'private, max-age=600, must-revalidate')
      return result
    }
  } else {
    return new Response("Country/Region not found: "+route.groups["collection"], { status: 404 })
  }
}

// BASE_URL and DATA will be defined during deployment process
