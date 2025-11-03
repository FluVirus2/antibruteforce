using System.Text.Json.Serialization;

namespace FluVirus.AntiBruteForce.Mock.Client.Cli;

public class Resource
{
    [JsonPropertyName("value")]
    public required int Value { get; init; }
}
