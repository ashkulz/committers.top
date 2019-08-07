## Most active GitHub users

This is a list of most active GitHub users in different countries/cities.
{% assign locations = (site.data.locations | sort:0) %}
{% for loc_hash in locations %}
  {% assign location = loc_hash[1] %}
  * [{{location.title}}]({{location.page}})
{% endfor %}
