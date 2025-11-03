using System.Text.Json.Serialization;

namespace FluVirus.AntiBruteForce.Mock.AuthServer.Api.Resources;

public class Resource
{
    [JsonPropertyName("value")]
    public required int Value { get; init; }
}
