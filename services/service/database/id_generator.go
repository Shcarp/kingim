package database

import "github.com/bwmarrin/snowflake"

type IDGenerator struct {
	node *snowflake.Node
}

// 雪花算法
func NewIDGenerator(nodeID int64)(*IDGenerator, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil,err
	}
	return &IDGenerator{node: node},nil
}

func (g *IDGenerator) Next() snowflake.ID {
	return g.node.Generate()
}

func (g*IDGenerator) ParseBase36(id string) (snowflake.ID, error) {
	return snowflake.ParseBase36(id)
}

func (g * IDGenerator) Parse(id int64) snowflake.ID {
	return snowflake.ParseInt64(id)

}
