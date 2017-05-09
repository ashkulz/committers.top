## Most active GitHub users

This is a list of most active GitHub users in different countries/cities.
{% for loc_hash in site.data.locations %}
  {% assign location = loc_hash[1] %}
  * [{{location.title}}]({{location.page}})
{% endfor %}
