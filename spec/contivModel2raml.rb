# This utility converts contiv object model definition in json format to raml format, which is essentially
# yaml with strict syntax.
require "json"
require "yaml"

class Hash
  def to_raml
    self.to_yaml.gsub("---", "#%RAML 1.0 Library")
  end
end

raise "you must supply an output file, e.g., #{$PROGRAM_NAME} ./foo.raml" unless ARGV.size == 1

outfile = ARGV[0]
output = Hash.new { |h, k| h[k] = {} }

Dir["../*.json"].each do |infile|

  puts infile

  data = JSON.parse(File.read(infile))

  object = data["objects"][0]

  properties = Hash.new { |h, k| h[k] = {} }

  if object["cfgProperties"]

    # for each entry under "cfgProperties", copy the relevant entries to the new
    # properties hash, changing key names as necessary.
    object["cfgProperties"].each do |name, value|
      p = properties[name]

      if value["type"] == "int"
        p["type"] = "integer"
      else
        p["type"] = value["type"]
      end

      if p["type"] == "array"
        p["items"] = {
          "type" => value["items"]
        }
      end

      if value["length"] && value["type"] == "string"
        p["maxLength"] = value["length"]
      end

      if value["title"]
        p["description"] = value["title"]
      end 

      if value["format"]
        p["pattern"] = value["format"]
      end

    end

  end

  # insert properties into desired output structure
  output["types"][object["name"]] = {
    "properties" => properties
  }

  output["types"]["#{object["name"]}s"] = {
    "type" => "array",
    "items" => {
      "type" => object["name"]
    }
  }

  # update structure, used with put/patch on main route
  #
  output["types"]["upd_#{object["name"]}"] = {
    "type" => object["name"]
  }

  # inspect structure, contains operational and config properties
  output["types"]["inspect_#{object["name"]}"] = {
    "properties" => {
      "Config" => {
        "type" => object["name"]
      }
    }
  }

  if object["operProperties"]

    opProperties = Hash.new { |h, k| h[k] = {} }

    object["operProperties"].each do |name, value|
      p = opProperties[name]

      if value["type"] == "int"
        p["type"] = "integer"
      else
        p["type"] = value["type"]
      end

      if p["type"] == "array"
        p["items"] = {
          "type" => value["items"]
        }
      end

      if value["length"] && value["type"] == "string"
        p["maxLength"] = value["length"]
      end

      if value["title"]
        p["description"] = value["title"]
      end

      if value["format"]
        p["pattern"] = value["format"]
      end

    end

    # add operational properties if present
    output["types"]["inspect_#{object["name"]}"]["properties"]["Oper"] = {
      "properties" => opProperties
    }

  end

end

File.write(outfile, output.to_raml)
