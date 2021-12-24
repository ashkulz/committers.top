## Most active GitHub users

This is a list of most active GitHub users in different countries/regions.
<ul class="country-list">
{% assign locations = site.data.locations | sort %}
{% for loc_hash in locations %}
  {% assign location = loc_hash[1] %}
  <li><a href="{{location.page | remove: '.html'}}">{{location.title}}</a></li>
{% endfor %}
</ul>

You can get a combined machine-readable JSON for:
<ul>
<li><a href="rank_only.json">rank-only with categories</a></li>
</ul>
A subset specific to each country/region is available on the individual page linked above.
